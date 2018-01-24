package collector

import (
	"io"
	"log"
	"path/filepath"
	"sync"

	"github.com/kubernetes-incubator/bootkube/e2e/internal/e2eutil/log-collector/pkg/fluentd"
	"github.com/kubernetes-incubator/bootkube/e2e/internal/e2eutil/log-collector/pkg/local"

	"k8s.io/client-go/kubernetes"
)

// Output interface describes where the log file is uploaded.
//
// Put accepts an 'io.ReadSeeker' of the log file and the 'filename' at the destination.
// Put returns the 'absolute (accessible) location' of the log file at the destination
// and an error if the file put was unsuccessful.
type Output interface {
	Put(io.ReadSeeker, string) (string, error)
}

// Collector provides functions to collect logs.
type Collector struct {
	scpConfig *scpConfig
	namespace string
	k8s       kubernetes.Interface

	Output Output
}

// Config defines configuration options for the Collector.
//
// Requires K8sClient and Namespce.
// If RemoteKeyFile is empty uses SSH_AUTH_SOCK to establish ssh connection.
// If RemoteUser is empty 'core' is used as default user for ssh connection.
// If RemotePort is empty '22' is used as default port for ssh connection.
type Config struct {
	RemoteUser    string
	RemotePort    int32
	RemoteKeyFile string
	K8sClient     kubernetes.Interface
	Namespace     string
}

// New returns *Collector given a *Config.
func New(c *Config) *Collector {
	return &Collector{
		scpConfig: &scpConfig{
			user:            c.RemoteUser,
			port:            c.RemotePort,
			identifyKeyFile: c.RemoteKeyFile,
		},
		namespace: c.Namespace,
		k8s:       c.K8sClient,
	}
}

// SetOutputToLocal sets the Collector output to local dir dstDir.
func (cr *Collector) SetOutputToLocal(dstDir string) error {
	l, err := local.New(&local.Config{
		Dir: dstDir,
	})
	if err != nil {
		return err
	}

	cr.Output = l
	return nil
}

// Start creates the fluentd master and worker assets for log collection.
// It also registers the node that is running the fluentd master.
func (cr *Collector) Start() error {
	if err := fluentd.CreateAssets(cr.k8s, cr.namespace); err != nil {
		return err
	}

	host, err := fluentd.GetNodeAddressWithMaster(cr.k8s, cr.namespace)
	if err != nil {
		return err
	}
	cr.scpConfig.host = host
	return nil
}

// CollectPodLogs fetches the log file(s) for the pods name matching basic shell file name pattern
// and uploads all the file(s).
// It returns the list locations where the log(s) were uploaded.
// for example, pattern 'kube-*' returns kube-apiserver, kube-scheduler etc.
// and 'apiserver' returns kube-apiserver.
func (cr *Collector) CollectPodLogs(pod string) ([]string, error) {
	scp, err := newScpClient(cr.scpConfig)
	if err != nil {
		return nil, err
	}
	defer scp.Close()

	paths, err := scp.GetPodsFilePaths(pod)
	if err != nil {
		return nil, err
	}

	results := upload(scp, cr.Output, paths)
	return results, nil
}

// CollectServiceLogs fetches the log file(s) for the services name matching basic shell file name pattern
// and uploads all the file(s).
// It returns the list locations where the log(s) were uploaded.
func (cr *Collector) CollectServiceLogs(service string) ([]string, error) {
	scp, err := newScpClient(cr.scpConfig)
	if err != nil {
		return nil, err
	}
	defer scp.Close()

	paths, err := scp.GetServicesFilePaths(service)
	if err != nil {
		return nil, err
	}

	results := upload(scp, cr.Output, paths)
	return results, nil
}

func upload(scp *scp, o Output, paths []string) []string {
	var results []string
	resp := make(chan string)
	go func() {
		for r := range resp {
			results = append(results, r)
		}
	}()

	maxGoroutines := 10
	var wg sync.WaitGroup
	guard := make(chan struct{}, maxGoroutines)

	for _, path := range paths {
		wg.Add(1)

		guard <- struct{}{}
		go func(fpath string, dst chan<- string) {
			defer wg.Done()
			defer func() {
				<-guard
			}()

			f, err := scp.Open(fpath)
			if err != nil {
				log.Printf("skipping fetch for file %s %v", fpath, err)
				return
			}
			defer f.Close()
			loc, err := o.Put(f, filepath.Base(f.Name()))
			if err != nil {
				log.Printf("skipping upload for file %s %v", fpath, err)
				return
			}
			dst <- loc
		}(path, resp)
	}

	wg.Wait()
	close(resp)
	close(guard)

	return results
}

// Cleanup deletes the fluentd assets and removes all annotations that were created on nodes.
func (cr *Collector) Cleanup() error {
	if err := fluentd.DeleteAssets(cr.k8s, cr.namespace); err != nil {
		return err
	}

	return nil
}
