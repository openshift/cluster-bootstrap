package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cmdRoot = &cobra.Command{
		Use:   "bootkube",
		Short: "Bootkube!",
		Long:  "",
	}
)

func main() {
	if err := cmdRoot.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
