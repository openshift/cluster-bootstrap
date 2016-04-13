package bootkube

import (
	"io/ioutil"
	"net"
	"time"

	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh"
	apiapp "k8s.io/kubernetes/cmd/kube-apiserver/app"
	apiserver "k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	controller "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	schedapp "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app"
	scheduler "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"
)

const (
	localAPIServerAddr         = "127.0.0.1:6443"
	localAPIServerInsecureAddr = "127.0.0.1:8080"
	remoteAPIServerAddr        = "0.0.0.0:6443"
	localEtcdServerAddr        = "127.0.0.1:2379"
	assetTimeout               = 5 * time.Minute
)

var requiredPods = []string{
	"kubelet",
	"kube-apiserver",
	"kube-scheduler",
	"kube-controller-manager",
}

type Config struct {
	SSHUser           string
	SSHKeyFile        string
	APIServerKey      string
	APIServerCert     string
	CACert            string
	ServiceAccountKey string
	TokenAuth         string
	RemoteAddr        string
	RemoteEtcdAddr    string
	ManifestDir       string
}

type bootkube struct {
	apiServer      *apiserver.APIServer
	controller     *controller.CMServer
	scheduler      *scheduler.SchedulerServer
	sshConfig      *ssh.ClientConfig
	remoteAddr     string
	remoteEtcdAddr string
	manifestDir    string
}

func NewBootkube(config Config) (*bootkube, error) {
	apiServer := apiserver.NewAPIServer()
	fs := pflag.NewFlagSet("apiserver", pflag.ExitOnError)
	apiServer.AddFlags(fs)
	fs.Parse([]string{
		"--allow-privileged=true",
		"--secure-port=6443",
		"--tls-private-key-file=" + config.APIServerKey,
		"--tls-cert-file=" + config.APIServerCert,
		"--etcd-servers=http://" + localEtcdServerAddr,
		"--service-cluster-ip-range=10.3.0.0/24",
		"--service-account-key-file=" + config.ServiceAccountKey,
		"--token-auth-file=" + config.TokenAuth,
		"--admission-control=ServiceAccount",
		"--runtime-config=extensions/v1beta1/deployments=true,extensions/v1beta1/daemonsets=true",
	})

	cmServer := controller.NewCMServer()
	fs = pflag.NewFlagSet("controllermanager", pflag.ExitOnError)
	cmServer.AddFlags(fs)
	fs.Parse([]string{
		"--master=" + localAPIServerInsecureAddr,
		"--service-account-private-key-file=" + config.ServiceAccountKey,
		"--root-ca-file=" + config.CACert,
		"--leader-elect=true",
	})

	schedServer := scheduler.NewSchedulerServer()
	fs = pflag.NewFlagSet("scheduler", pflag.ExitOnError)
	schedServer.AddFlags(fs)
	fs.Parse([]string{
		"--master=" + localAPIServerInsecureAddr,
		"--leader-elect=true",
	})

	sshConfig, err := newSSHConfig(config.SSHUser, config.SSHKeyFile)
	if err != nil {
		return nil, err
	}

	return &bootkube{
		apiServer:      apiServer,
		controller:     cmServer,
		scheduler:      schedServer,
		sshConfig:      sshConfig,
		remoteAddr:     config.RemoteAddr,
		remoteEtcdAddr: config.RemoteEtcdAddr,
		manifestDir:    config.ManifestDir,
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
		listenAddr: remoteAPIServerAddr,
		listenFunc: tunnel.Listen,
		dialAddr:   localAPIServerAddr,
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
		if err := CreateAssets(b.manifestDir, assetTimeout); err != nil {
			errch <- err
		}
	}()
	go func() { errch <- WaitUntilPodsRunning(requiredPods, assetTimeout) }()

	// If any of the bootkube services exit, it means it is unrecoverable and we should exit.
	return <-errch
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
