package main

import (
	"errors"
	"strings"

	"github.com/openshift/cluster-bootstrap/pkg/script"
	"github.com/spf13/cobra"

	"github.com/openshift/cluster-bootstrap/pkg/start"
)

var (
	cmdScript = &cobra.Command{
		Use:          "script",
		Short:        "Print the bootstrap script to stdout",
		Long:         "",
		PreRunE:      validateScriptOpts,
		RunE:         runCmdScript,
		SilenceUsage: true,
	}

	scriptOpts struct {
		releaseImageDigest string
		templatePath       string
		etcdServerURLs     []string
	}
)

func init() {
	cmdRoot.AddCommand(cmdScript)
	cmdScript.Flags().StringVar(&scriptOpts.releaseImageDigest, "release-image-digest", "", "Image digest for the release image.")
	cmdScript.Flags().StringVar(&scriptOpts.templatePath, "template-path", "/bootkube.sh", "Script template path.")
	cmdScript.Flags().MarkHidden("template-path")
	cmdScript.Flags().StringSliceVar(&scriptOpts.etcdServerURLs, "etcd-server-urls", nil, "The etcd server URL, comma separated.")
}

func runCmdScript(cmd *cobra.Command, args []string) error {
	bk, err := script.NewScriptCommand(script.Config{
		ScriptPath:         scriptOpts.templatePath,
		ReleaseImageDigest: scriptOpts.releaseImageDigest,
		EtcdCluster:        strings.Join(scriptOpts.etcdServerURLs, ","),
	})
	if err != nil {
		return err
	}

	err = bk.Run()
	if err != nil {
		// Always report errors.
		start.UserOutput("Error: %v\n", err)
	}
	return err
}

func validateScriptOpts(cmd *cobra.Command, args []string) error {
	if len(scriptOpts.releaseImageDigest) == 0 {
		return errors.New("missing required flag: --release-image-digest")
	}
	if len(scriptOpts.etcdServerURLs) == 0 {
		return errors.New("missing required flag: --etcd-server-urls")
	}
	return nil
}
