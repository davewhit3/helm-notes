package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/output"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
)

var settings = cli.New()

const notesDesc = `
This command render a chart NOTES.txt file.

The notes argument must be a chart reference, a path to a packaged chart,
a path to an unpacked chart directory or a URL.

To override values in a chart, use either the '--values' flag and pass in a file
or use the '--set' flag and pass configuration from the command line, to force
a string value use '--set-string'. You can use '--set-file' to set individual
values from a file when the value itself is too long for the command line
or is dynamically generated.

    $ helm notes -f myvalues.yaml myredis ./redis

or

    $ helm notes --set name=prod myredis ./redis

or

    $ helm notes --set-string long_int=1234567890 myredis ./redis

or

    $ helm notes --set-file my_script=dothings.sh myredis ./redis

You can specify the '--values'/'-f' flag multiple times. The priority will be given to the
last (right-most) file specified. For example, if both myvalues.yaml and override.yaml
contained a key called 'Test', the value set in override.yaml would take precedence:

    $ helm notes -f myvalues.yaml -f override.yaml  myredis ./redis

You can specify the '--set' flag multiple times. The priority will be given to the
last (right-most) set specified. For example, if both 'bar' and 'newbar' values are
set for a key called 'foo', the 'newbar' value would take precedence:

    $ helm notes --set foo=bar --set foo=newbar  myredis ./redis


1. By chart reference: helm notes mymaria example/mariadb
2. By path to a packaged chart: helm notes mynginx ./nginx-1.2.3.tgz
3. By path to an unpacked chart directory: helm notes mynginx ./nginx
4. By absolute URL: helm notes mynginx https://example.com/charts/nginx-1.2.3.tgz
5. By chart reference and repo url: helm notes --repo https://example.com/charts/ mynginx nginx

CHART REFERENCES

A chart reference is a convenient way of referencing a chart in a chart repository.

When you use a chart reference with a repo prefix ('example/mariadb'), Helm will look in the local
configuration for a chart repository named 'example', and will then look for a
chart in that repository whose name is 'mariadb'. It will install the latest stable version of that chart
until you specify '--devel' flag to also include development version (alpha, beta, and release candidate releases), or
supply a version number with the '--version' flag.

To see the list of chart repositories, use 'helm repo list'. To search for
charts in a repository, use 'helm search'.
`

func newRenderCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewInstall(cfg)
	valueOpts := &values.Options{}
	var outfmt output.Format

	cmd := &cobra.Command{
		Use:   "[NAME] [CHART]",
		Short: "render a chart NOTES.txt file",
		Long:  notesDesc,
		Args:  require.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return compInstall(args, toComplete, client)
		},
		RunE: func(_ *cobra.Command, args []string) error {
			rel, err := runRender(args, client, valueOpts, out)
			if err != nil {
				return errors.Wrap(err, "RENDER FAILED")
			}

			return outfmt.Write(out, &statusPrinter{rel, settings.Debug, false})
		},
	}

	addInstallFlags(cmd, cmd.Flags(), client, valueOpts)
	bindOutputFlag(cmd, &outfmt)
	bindPostRenderFlag(cmd, &client.PostRenderer)

	return cmd
}

func addInstallFlags(cmd *cobra.Command, f *pflag.FlagSet, client *action.Install, valueOpts *values.Options) {
	f.BoolVar(&client.DisableHooks, "no-hooks", false, "prevent hooks from running during install")
	f.BoolVar(&client.Replace, "replace", false, "re-use the given name, only if that name is a deleted release which remains in the history. This is unsafe in production")
	f.DurationVar(&client.Timeout, "timeout", 300*time.Second, "time to wait for any individual Kubernetes operation (like Jobs for hooks)")
	f.BoolVarP(&client.GenerateName, "generate-name", "g", false, "generate the name (and omit the NAME parameter)")
	f.StringVar(&client.NameTemplate, "name-template", "", "specify template used to name the release")
	f.StringVar(&client.Description, "description", "", "add a custom description")
	f.BoolVar(&client.Devel, "devel", false, "use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored")
	f.BoolVar(&client.DependencyUpdate, "dependency-update", false, "update dependencies if they are missing before installing the chart")
	f.BoolVar(&client.DisableOpenAPIValidation, "disable-openapi-validation", false, "if set, the installation process will not validate rendered templates against the Kubernetes OpenAPI Schema")
	f.BoolVar(&client.Atomic, "atomic", false, "if set, the installation process deletes the installation on failure. The --wait flag will be set automatically if --atomic is used")
	f.BoolVar(&client.SkipCRDs, "skip-crds", false, "if set, no CRDs will be installed. By default, CRDs are installed if not already present")
	f.BoolVar(&client.SubNotes, "render-subchart-notes", false, "if set, render subchart notes along with the parent")
	addValueOptionsFlags(f, valueOpts)
	addChartPathOptionsFlags(f, &client.ChartPathOptions)

	err := cmd.RegisterFlagCompletionFunc("version", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		requiredArgs := 2
		if client.GenerateName {
			requiredArgs = 1
		}
		if len(args) != requiredArgs {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return compVersionFlag(args[requiredArgs-1], toComplete)
	})

	if err != nil {
		log.Fatal(err)
	}
}

func runRender(args []string, client *action.Install, valueOpts *values.Options, out io.Writer) (*release.Release, error) {
	if client.Version == "" && client.Devel {
		client.Version = ">0.0.0-0"
	}

	name, chart, err := client.NameAndChart(args)
	if err != nil {
		return nil, err
	}
	client.ReleaseName = name

	cp, err := client.ChartPathOptions.LocateChart(chart, settings)
	if err != nil {
		return nil, err
	}

	p := getter.All(settings)
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return nil, err
	}

	// Check chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(cp)
	if err != nil {
		return nil, err
	}

	if err := checkIfInstallable(chartRequested); err != nil {
		return nil, err
	}

	if req := chartRequested.Metadata.Dependencies; req != nil {
		// If CheckDependencies returns an error, we have unfulfilled dependencies.
		// As of Helm 2.4.0, this is treated as a stopping condition:
		// https://github.com/helm/helm/issues/2209
		if err := action.CheckDependencies(chartRequested, req); err != nil {
			err = errors.Wrap(err, "An error occurred while checking for chart dependencies. You may need to run `helm dependency build` to fetch missing dependencies")
			if client.DependencyUpdate {
				man := &downloader.Manager{
					Out:              out,
					ChartPath:        cp,
					Keyring:          client.ChartPathOptions.Keyring,
					SkipUpdate:       false,
					Getters:          p,
					RepositoryConfig: settings.RepositoryConfig,
					RepositoryCache:  settings.RepositoryCache,
					Debug:            settings.Debug,
				}
				if err := man.Update(); err != nil {
					return nil, err
				}
				// Reload the chart with the updated Chart.lock file.
				if chartRequested, err = loader.Load(cp); err != nil {
					return nil, errors.Wrap(err, "failed reloading chart after repo update")
				}
			} else {
				return nil, err
			}
		}
	}

	client.Namespace = settings.Namespace()

	// Create context and prepare the handle of SIGTERM
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	// Set up channel on which to send signal notifications.
	// We must use a buffered channel or risk missing the signal
	// if we're not ready to receive when the signal is sent.
	cSignal := make(chan os.Signal, 2)
	signal.Notify(cSignal, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-cSignal
		fmt.Fprintf(out, "Release %s has been cancelled.\n", args[0])
		cancel()
	}()

	return client.RunWithContext(ctx, chartRequested, vals)
}

// checkIfInstallable validates if a chart can be installed
//
// Application chart type is only installable
func checkIfInstallable(ch *chart.Chart) error {
	switch ch.Metadata.Type {
	case "", "application":
		return nil
	}
	return errors.Errorf("%s charts are not installable", ch.Metadata.Type)
}

// Provide dynamic auto-completion for the install and template commands
func compInstall(args []string, toComplete string, client *action.Install) ([]string, cobra.ShellCompDirective) {
	requiredArgs := 1
	if client.GenerateName {
		requiredArgs = 0
	}
	if len(args) == requiredArgs {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}
