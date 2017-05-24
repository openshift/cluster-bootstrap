package etcdutil

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/wait"
)

// WaitClusterReady waits the etcd server ready to serve client requests.
func WaitClusterReady(endpoint string) error {
	err := wait.Poll(pollInterval, pollTimeout, func() (bool, error) {
		resp, err := http.Get(fmt.Sprintf("http://%s/version", endpoint))
		if err != nil {
			glog.Infof("could not read from etcd: %v", err)
			return false, nil
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if len(body) == 0 || err != nil {
			glog.Infof("could not read from etcd: %v", err)
			return false, nil
		}
		return true, nil
	})

	return err
}
