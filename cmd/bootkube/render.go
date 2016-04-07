package main

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/coreos/bootkube/pkg/asset"
)

var (
	cmdRender = &cobra.Command{
		Use:     "render",
		Short:   "Render default cluster manifests",
		Long:    "",
		PreRunE: validateRenderOpts,
		RunE:    runCmdRender,
	}

	outDir string
)

func init() {
	cmdRoot.AddCommand(cmdRender)
	cmdRender.Flags().StringVar(&outDir, "outdir", "", "Output path for rendered manifests")
}

func runCmdRender(cmd *cobra.Command, args []string) error {
	as, err := asset.NewDefaultAssets()
	if err != nil {
		return err
	}
	return as.WriteFiles(outDir)
}

func validateRenderOpts(cmd *cobra.Command, args []string) error {
	if outDir == "" {
		return errors.New("Missing required flag: --outdir")
	}
	return nil
}
