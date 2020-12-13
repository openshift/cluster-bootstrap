package main

import (
	"errors"
	"github.com/openshift/cluster-bootstrap/pkg/ibip"
	"github.com/spf13/cobra"

)

var (
	cmdIBip = &cobra.Command{
		Use:          "ibip",
		Short:        "Update the master ignition with control plane static pods files",
		Long:         "",
		PreRunE:      validateIBipOpts,
		RunE:         runCmdIBip,
		SilenceUsage: true,
	}

	iBipOpts struct {
		assetDir             string
		ignitionPath		 string
	}
)

func init() {
	cmdRoot.AddCommand(cmdIBip)
	cmdIBip.Flags().StringVar(&iBipOpts.assetDir, "asset-dir", "", "Path to the cluster asset directory.")
	cmdIBip.Flags().StringVar(&iBipOpts.ignitionPath, "ignition-path", "/assets/master.ign", "The location of master ignition")

}

func runCmdIBip(cmd *cobra.Command, args []string) error {

	ib, err := ibip.NewIBipCommand(ibip.ConfigIBip{
		AssetDir:             iBipOpts.assetDir,
		IgnitionPath:      iBipOpts.ignitionPath,
	})
	if err != nil {
		return err
	}

	return ib.UpdateSnoIgnitionData()
}

func validateIBipOpts(cmd *cobra.Command, args []string) error {
	if iBipOpts.ignitionPath == "" {
		return errors.New("missing required flag: --ignition-path")
	}
	if iBipOpts.assetDir == "" {
		return errors.New("missing required flag: --asset-dir")
	}
	return nil
}
