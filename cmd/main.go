package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/util"

	"github.com/coreos/bootkube/pkg/bootkube"
)

func main() {
	var opts bootkube.Opts

	fs := pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)

	fs.StringVar(&opts.SSHUser, "ssh-user", "", "Username for the remote ssh host")
	fs.StringVar(&opts.SSHKeyFile, "ssh-keyfile", "", "Keyfile for the remote ssh user")
	fs.StringVar(&opts.RemoteAddr, "remote-address", "", "Remote ssh host (ip:port)")
	fs.StringVar(&opts.RemoteEtcdAddr, "remote-etcd-address", "127.0.0.1:2379", "Remote etcd host (ip:port)")
	fs.StringVar(&opts.AssetDir, "manifest-dir", "", "Path to Kubernetes object manifests")
	fs.Parse(os.Args[1:])

	if err := validateOpts(opts); err != nil {
		fail(err)
	}

	bk, err := bootkube.NewBootkube(opts)
	if err != nil {
		fail(err)
	}

	util.InitLogs()
	defer util.FlushLogs()

	if err := bk.Run(); err != nil {
		fail(err)
	}
}

func validateOpts(opts bootkube.Opts) error {
	if opts.SSHUser == "" {
		return errors.New("must specify an ssh-user")
	}
	if opts.SSHKeyFile == "" {
		return errors.New("must specify an ssh-keyfile")
	}
	if opts.RemoteAddr == "" {
		return errors.New("must specify a remote-address")
	}
	return nil
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
