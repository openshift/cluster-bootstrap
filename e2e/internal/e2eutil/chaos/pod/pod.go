// Copyright 2016 The etcd-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package chaos

import (
	"context"
	"math/rand"

	"github.com/golang/glog"
	"golang.org/x/time/rate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// Monkey knows how to crush pods.
type Monkey struct {
	kubecli kubernetes.Interface
}

// NewMonkey creates a Monkey to crush pods.
func NewMonkey(kubecli kubernetes.Interface) *Monkey {
	return &Monkey{kubecli: kubecli}
}

type CrushConfig struct {
	// Namespace is the namespace of the pods to crush.
	Namespace string
	// Selector selects pods to crush.
	Selector labels.Selector

	// KillRate is the rate to crush pods.
	KillRate rate.Limit
	// KillProbability is the probability to crush selected pods.
	KillProbability float64
	// KillMax is the max number of selected pods to crush.
	KillMax int
}

// TODO: respect context in k8s operations.
func (m *Monkey) CrushPods(ctx context.Context, c *CrushConfig) {
	burst := int(c.KillRate)
	if burst <= 0 {
		burst = 1
	}
	limiter := rate.NewLimiter(c.KillRate, burst)
	ls := c.Selector.String()
	ns := c.Namespace
	for {
		err := limiter.Wait(ctx)
		if err != nil { // user cancellation
			glog.V(4).Infof("crushPods is canceled for selector %v by the user: %v", ls, err)
			return
		}

		if p := rand.Float64(); p > c.KillProbability {
			glog.V(4).Infof("skip killing pod: probability: %v, got p: %v", c.KillProbability, p)
			continue
		}

		pods, err := m.kubecli.CoreV1().Pods(ns).List(metav1.ListOptions{LabelSelector: ls})
		if err != nil {
			glog.Errorf("failed to list pods for selector %v: %v", ls, err)
			continue
		}
		if len(pods.Items) == 0 {
			glog.V(4).Infof("no pods to kill for selector %v", ls)
			continue
		}

		max := len(pods.Items)
		kmax := rand.Intn(c.KillMax) + 1
		if kmax < max {
			max = kmax
		}

		glog.V(4).Infof("start to kill %d pods for selector %v", max, ls)

		tokills := make(map[string]struct{})
		for len(tokills) < max {
			tokills[pods.Items[rand.Intn(len(pods.Items))].Name] = struct{}{}
		}

		for tokill := range tokills {
			err = m.kubecli.CoreV1().Pods(ns).Delete(tokill, metav1.NewDeleteOptions(0))
			if err != nil {
				glog.V(4).Infof("failed to kill pod %v: %v", tokill, err)
				continue
			}
			glog.V(4).Infof("killed pod %v for selector %v", tokill, ls)
		}
	}
}
