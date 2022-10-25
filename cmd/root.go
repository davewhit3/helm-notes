package cmd

import (
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
)

const rootCmdLongUsage = `
The Helm Notes Plugin
`

// New creates a new cobra client
func New() *cobra.Command {
	registryClient, _ := registry.NewClient()
	conf := &action.Configuration{
		Releases:       storage.Init(driver.NewMemory()),
		KubeClient:     &kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: ioutil.Discard}},
		Capabilities:   chartutil.DefaultCapabilities,
		RegistryClient: registryClient,
		Log: func(format string, v ...interface{}) {

		},
	}
	cmd := newInstallCmd(conf, os.Stdout)
	// cmd.Use = "helm notes"
	// cmd.Short = "Render notes.txt"
	// cmd.Long = rootCmdLongUsage

	// cmd.SetHelpCommand(&cobra.Command{}) // Disable the help command
	return cmd
}

type chartOptions struct {
	*chart.Chart
}

type chartOption func(*chartOptions)

func buildChart(opts ...chartOption) *chart.Chart {
	c := &chartOptions{
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				APIVersion: "v1",
				Name:       "hello",
				Version:    "0.1.0",
			},
			Templates: []*chart.File{
				{Name: "templates/NOTES.txt", Data: []byte("asd")},
			},
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c.Chart
}
