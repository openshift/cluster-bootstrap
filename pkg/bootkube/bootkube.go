package bootkube

import (
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh"
	apiapp "k8s.io/kubernetes/cmd/kube-apiserver/app"
	apiserver "k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	controller "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/wait"
	schedapp "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app"
	scheduler "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"
)

const (
	localAPIServerAddr  = "127.0.0.1:8080"
	remoteAPIServerAddr = "127.0.0.1:8080"
	localEtcdServerAddr = "127.0.0.1:2379"
)

type Opts struct {
	SSHUser        string
	SSHKeyFile     string
	RemoteAddr     string
	RemoteEtcdAddr string
	AssetDir       string
}

type bootkube struct {
	apiServer      *apiserver.APIServer
	controller     *controller.CMServer
	scheduler      *scheduler.SchedulerServer
	clientConfig   clientcmd.ClientConfig
	sshConfig      *ssh.ClientConfig
	remoteAddr     string
	remoteEtcdAddr string
	assetDir       string
}

func NewBootkube(opts Opts) (*bootkube, error) {
	apiServer := apiserver.NewAPIServer()
	fs := pflag.NewFlagSet("apiserver", pflag.ExitOnError)
	apiServer.AddFlags(fs)
	fs.Parse([]string{
		"--etcd-servers=http://" + localEtcdServerAddr,
		"--service-cluster-ip-range=10.3.0.0/24",
	})

	cmServer := controller.NewCMServer()
	fs = pflag.NewFlagSet("controllermanager", pflag.ExitOnError)
	cmServer.AddFlags(fs)
	fs.Parse([]string{
		"--master=" + localAPIServerAddr,
	})

	schedServer := scheduler.NewSchedulerServer()
	fs = pflag.NewFlagSet("scheduler", pflag.ExitOnError)
	schedServer.AddFlags(fs)
	fs.Parse([]string{
		"--master=http://" + localAPIServerAddr,
	})

	sshConfig, err := newSSHConfig(opts.SSHUser, opts.SSHKeyFile)
	if err != nil {
		return nil, err
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: localAPIServerAddr}},
	)

	return &bootkube{
		apiServer:      apiServer,
		controller:     cmServer,
		scheduler:      schedServer,
		clientConfig:   clientConfig,
		sshConfig:      sshConfig,
		remoteAddr:     opts.RemoteAddr,
		remoteEtcdAddr: opts.RemoteEtcdAddr,
		assetDir:       opts.AssetDir,
	}, nil
}

func (b *bootkube) Run() error {
	tunnel, err := ssh.Dial("tcp", b.remoteAddr, b.sshConfig)
	if err != nil {
		return err
	}
	defer tunnel.Close()

	// Proxy connections from remote end of ssh tunnel to local apiserver
	apiProxy := &Proxy{
		listenAddr: localAPIServerAddr,
		listenFunc: tunnel.Listen,
		dialAddr:   remoteAPIServerAddr,
		dialFunc:   net.Dial,
	}

	// Proxy local connections to remote etcd via ssh tunnel
	etcdProxy := &Proxy{
		listenAddr: localEtcdServerAddr,
		listenFunc: net.Listen,
		dialAddr:   b.remoteEtcdAddr,
		dialFunc:   tunnel.Dial,
	}

	errch := make(chan error)
	go func() { errch <- apiProxy.Run() }()
	go func() { errch <- etcdProxy.Run() }()
	go func() { errch <- apiapp.Run(b.apiServer) }()
	go func() { errch <- cmapp.Run(b.controller) }()
	go func() { errch <- schedapp.Run(b.scheduler) }()
	go func() {
		if err := b.createAssets(); err != nil {
			//TODO(aaron): we should allow "already exists" errors
			errch <- err
		}
	}()

	// If any of the bootkube services exit, it means they can't initialize and we should exit.
	return <-errch
}

func (b *bootkube) createAssets() error {
	if err := wait.Poll(5*time.Second, 60*time.Second, b.isUp); err != nil {
		return fmt.Errorf("API Server unavailable: %v", err)
	}

	f := cmdutil.NewFactory(b.clientConfig)
	schema, err := f.Validator(true, fmt.Sprintf("~/%s/%s", clientcmd.RecommendedHomeDir, clientcmd.RecommendedSchemaName))
	if err != nil {
		return err
	}
	cmdNamespace, enforceNamespace, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	mapper, typer := f.Object()
	r := resource.NewBuilder(mapper, typer, resource.ClientMapperFunc(f.ClientForMapping), f.Decoder(true)).
		Schema(schema).
		ContinueOnError().
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(enforceNamespace, b.assetDir).
		Flatten().
		Do()
	err = r.Err()
	if err != nil {
		return err
	}

	count := 0
	err = r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		obj, err := resource.NewHelper(info.Client, info.Mapping).Create(info.Namespace, true, info.Object)
		if err != nil {
			return cmdutil.AddSourceToErr("creating", info.Source, err)
		}
		info.Refresh(obj, true)

		count++
		cmdutil.PrintSuccess(mapper, false, util.GlogWriter{}, info.Mapping.Resource, info.Name, "created")
		return nil
	})
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no objects passed to create")
	}
	return nil

}

func (b *bootkube) isUp() (bool, error) {
	f := cmdutil.NewFactory(b.clientConfig)
	client, err := f.Client()
	if err != nil {
		return false, fmt.Errorf("failed to create kube client: %v", err)
	}

	_, err = client.Discovery().ServerVersion()
	return err == nil, err
}

func newSSHConfig(user, keyfile string) (*ssh.ClientConfig, error) {
	buffer, err := ioutil.ReadFile(keyfile)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil, err
	}

	return &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
	}, nil
}
