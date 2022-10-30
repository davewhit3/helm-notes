package cmd

import (
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
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
		Log:            func(format string, v ...interface{}) {},
	}
	cmd := newRenderCmd(conf, os.Stdout)
	cmd.AddCommand(newVersionCmd())

	cmd.SetHelpCommand(&cobra.Command{}) // Disable the help command
	return cmd
}
