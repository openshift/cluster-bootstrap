package main

import (
	"errors"

	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/util"

	"github.com/coreos/bootkube/pkg/bootkube"
)

var (
	cmdStart = &cobra.Command{
		Use:          "start",
		Short:        "Start the bootkube service",
		Long:         "",
		PreRunE:      validateStartOpts,
		RunE:         runCmdStart,
		SilenceUsage: true,
	}

	opts = bootkube.Config{}
)

func init() {
	cmdRoot.AddCommand(cmdStart)
	cmdStart.Flags().StringVar(&opts.SSHUser, "ssh-user", "", "Username for the remote ssh host")
	cmdStart.Flags().StringVar(&opts.SSHKeyFile, "ssh-keyfile", "", "Keyfile for the remote ssh user")
	cmdStart.Flags().StringVar(&opts.APIServerKey, "apiserver-key", "", "API server private key")
	cmdStart.Flags().StringVar(&opts.APIServerCert, "apiserver-cert", "", "API server certificate")
	cmdStart.Flags().StringVar(&opts.CACert, "ca-cert", "", "CA certificate")
	cmdStart.Flags().StringVar(&opts.ServiceAccountKey, "service-account-key", "", "Service account key")
	cmdStart.Flags().StringVar(&opts.TokenAuth, "token-auth-file", "", "token auth csv file")
	cmdStart.Flags().StringVar(&opts.RemoteAddr, "remote-address", "", "Remote ssh host (ip:port)")
	cmdStart.Flags().StringVar(&opts.RemoteEtcdAddr, "remote-etcd-address", "127.0.0.1:2379", "Remote etcd host (ip:port)")
	cmdStart.Flags().StringVar(&opts.ManifestDir, "manifest-dir", "", "Path to Kubernetes object manifests")
}

func runCmdStart(cmd *cobra.Command, args []string) error {
	bk, err := bootkube.NewBootkube(opts)
	if err != nil {
		return err
	}

	util.InitLogs()
	defer util.FlushLogs()

	return bk.Run()
}

func validateStartOpts(cmd *cobra.Command, args []string) error {
	if opts.SSHUser == "" {
		return errors.New("must specify an ssh-user")
	}
	if opts.SSHKeyFile == "" {
		return errors.New("must specify an ssh-keyfile")
	}
	if opts.APIServerKey == "" {
		return errors.New("must specify an apiserver-key")
	}
	if opts.APIServerCert == "" {
		return errors.New("must specify an apiserver-cert")
	}
	if opts.CACert == "" {
		return errors.New("must specify a ca-cert")
	}
	if opts.ServiceAccountKey == "" {
		return errors.New("must specify a service-account-key")
	}
	if opts.TokenAuth == "" {
		return errors.New("must specify a token-auth-file")
	}
	if opts.RemoteAddr == "" {
		return errors.New("must specify a remote-address")
	}
	return nil
}
