package main

import (
	"errors"

	"github.com/openshift/cluster-bootstrap/pkg/bootstrapinplace"

	"github.com/spf13/cobra"
)

var (
	CmdBootstrapInPlace = &cobra.Command{
		Use:          "bootstrap-in-place",
		Short:        "Create Ignition based on Fedora CoreOS Config",
		Long:         "",
		PreRunE:      validateBootstrapInPlaceOpts,
		RunE:         runCmdBootstrapInPlace,
		SilenceUsage: true,
	}

	bootstrapInPlaceOpts struct {
		assetDir     string
		ignitionPath string
		input        string
		Pretty       bool
	}
)

func init() {
	cmdRoot.AddCommand(CmdBootstrapInPlace)
	CmdBootstrapInPlace.Flags().BoolVarP(&bootstrapInPlaceOpts.Pretty, "pretty", "p", true, "output formatted json")
	CmdBootstrapInPlace.Flags().StringVar(&bootstrapInPlaceOpts.input, "input", "", "fcc input file path")
	CmdBootstrapInPlace.Flags().StringVar(&bootstrapInPlaceOpts.ignitionPath, "output", "o", "Ignition output file path")
	CmdBootstrapInPlace.Flags().StringVarP(&bootstrapInPlaceOpts.assetDir, "asset-dir", "d", "", "allow embedding local files from this directory")
}

func runCmdBootstrapInPlace(cmd *cobra.Command, args []string) error {
	bip, err := bootstrapinplace.NewBootstrapInPlaceCommand(bootstrapinplace.BootstrapInPlaceConfig{
		AssetDir:     bootstrapInPlaceOpts.assetDir,
		IgnitionPath: bootstrapInPlaceOpts.ignitionPath,
		Input:        bootstrapInPlaceOpts.input,
		Pretty:       bootstrapInPlaceOpts.Pretty,
	})
	if err != nil {
		return err
	}

	return bip.Create()
}

func validateBootstrapInPlaceOpts(cmd *cobra.Command, args []string) error {
	if bootstrapInPlaceOpts.ignitionPath == "" {
		return errors.New("missing required flag: --output")
	}
	if bootstrapInPlaceOpts.assetDir == "" {
		return errors.New("missing required flag: --asset-dir")
	}
	if bootstrapInPlaceOpts.input == "" {
		return errors.New("missing required flag: --input")
	}
	return nil
}
