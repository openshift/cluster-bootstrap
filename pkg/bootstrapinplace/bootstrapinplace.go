// Copyright 2019 Red Hat, Inc

package bootstrapinplace

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/coreos/butane/config"
	butaneCommon "github.com/coreos/butane/config/common"
)

func fail(format string, args ...interface{}) {
	fmt.Printf(format, args...)
	os.Exit(1)
}

type BootstrapInPlaceConfig struct {
	AssetDir     string
	IgnitionPath string
	Input        string
	Strict       bool
	Pretty       bool
}
type BootstrapInPlaceCommand struct {
	config BootstrapInPlaceConfig
}

func NewBootstrapInPlaceCommand(config BootstrapInPlaceConfig) (*BootstrapInPlaceCommand, error) {
	return &BootstrapInPlaceCommand{
		config: config,
	}, nil
}

// Creating master ignition that will be used by node after reboot
// Using butane tool (tool that takes yaml and according to it creates ignition):
// 1. Read actions yaml that has all the data needed by butane to create master.ign
// 2. Create ignition data
// 3. Write created data to file
func (i *BootstrapInPlaceCommand) Create() error {
	infile, err := os.Open(i.config.Input)
	if err != nil {
		fail("Error occurred while trying to open %s: %v\n", i.config.Input, err)
	}
	defer infile.Close()

	dataIn, err := ioutil.ReadAll(infile)
	if err != nil {
		fail("Error occurred while trying to read %s: %v\n", infile.Name(), err)
	}

	dataOut, r, err := config.TranslateBytes(dataIn, butaneCommon.TranslateBytesOptions{
		TranslateOptions: butaneCommon.TranslateOptions{FilesDir: i.config.AssetDir},
		Pretty:           i.config.Pretty,
		Strict:           i.config.Strict},
	)
	fmt.Println(r.String())
	if err != nil {
		fail("Error translating config: %v\n", err)
	}

	outfile, err := os.OpenFile(i.config.IgnitionPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		fail("Failed to open %s: %v\n", i.config.IgnitionPath, err)
	}
	defer outfile.Close()

	if _, err := outfile.Write(append(dataOut, '\n')); err != nil {
		fail("Failed to write config to %s: %v\n", outfile.Name(), err)
	}
	return nil
}
