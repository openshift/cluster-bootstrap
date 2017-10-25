package checkpoint

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"
)

func TestProcess(t *testing.T) {
	type testCase struct {
		desc                string
		localRunning        map[string]*v1.Pod
		localParents        map[string]*v1.Pod
		apiParents          map[string]*v1.Pod
		activeCheckpoints   map[string]*v1.Pod
		inactiveCheckpoints map[string]*v1.Pod
		expectStart         []string
		expectStop          []string
		expectRemove        []string
		podName             string
	}

	cases := []testCase{
		{
			desc:                "Inactive checkpoint and no local running: should start",
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			expectStart:         []string{"AA"},
		},
		{
			desc:                "Inactive checkpoint and local running: no change",
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			localRunning:        map[string]*v1.Pod{"AA": {}},
		},
		{
			desc:                "Inactive checkpoint and no api parent: should remove",
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			apiParents:          map[string]*v1.Pod{"BB": {}},
			expectRemove:        []string{"AA"},
		},
		{
			desc:                "Inactive checkpoint and both api & local running: no change",
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			localRunning:        map[string]*v1.Pod{"AA": {}},
			apiParents:          map[string]*v1.Pod{"AA": {}},
		},
		{
			desc:                "Inactive checkpoint and only api parent: should start",
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			apiParents:          map[string]*v1.Pod{"AA": {}},
			expectStart:         []string{"AA"},
		},
		{
			desc:              "Active checkpoint and no local running: no change",
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
		},
		{
			desc:              "Active checkpoint and local running: should stop",
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
			localRunning:      map[string]*v1.Pod{"AA": {}},
			expectStop:        []string{"AA"},
		},
		{
			desc:              "Active checkpoint and api parent: no change",
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
			apiParents:        map[string]*v1.Pod{"AA": {}},
		},
		{
			desc:              "Active checkpoint and no api parent: remove",
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
			apiParents:        map[string]*v1.Pod{"BB": {}},
			expectRemove:      []string{"AA"},
		},
		{
			desc:              "Active checkpoint with local running, and api parent: should stop",
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
			localRunning:      map[string]*v1.Pod{"AA": {}},
			apiParents:        map[string]*v1.Pod{"AA": {}},
			expectStop:        []string{"AA"},
		},
		{
			desc:              "Active checkpoint with local parent, and no api parent: should remove",
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
			localParents:      map[string]*v1.Pod{"AA": {}},
			apiParents:        map[string]*v1.Pod{"BB": {}},
			expectRemove:      []string{"AA"},
		},
		{
			desc:                "Both active and inactive checkpoints, with no api parent: remove both",
			activeCheckpoints:   map[string]*v1.Pod{"AA": {}},
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			apiParents:          map[string]*v1.Pod{"BB": {}},
			expectRemove:        []string{"AA"}, // Only need single remove, we should clean up both active/inactive
		},
		{
			desc:                "Inactive checkpoint, local parent, local running, no api parent: no change", // Safety check - don't GC if local parent still exists (even if possibly stale)
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			localRunning:        map[string]*v1.Pod{"AA": {}},
			localParents:        map[string]*v1.Pod{"AA": {}},
		},
		{
			desc:              "Active checkpoint, local parent, no local running, no api parent: no change", // Safety check - don't GC if local parent still exists (even if possibly stale)
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
			localParents:      map[string]*v1.Pod{"AA": {}},
		},
		{
			desc:         "Inactive pod-checkpointer, local parent, local running, api parent: should start",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			localParents: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			apiParents:   map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
			},
			expectStart: []string{"kube-system/pod-checkpointer"},
		},
		{
			desc:         "Inactive pod-checkpointer, local parent, no local running, api not reachable: should start",
			localParents: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
			},
			expectStart: []string{"kube-system/pod-checkpointer"},
		},
		{
			desc:         "Inactive pod-checkpointer, no local parent, no api parent: should remove in the last",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}, "AA": {}},
			localParents: map[string]*v1.Pod{"BB": {}},
			apiParents:   map[string]*v1.Pod{"BB": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
				"AA": {},
			},
			expectRemove: []string{"AA", "kube-system/pod-checkpointer"},
		},
		{
			desc:         "Inactive pod-checkpointer, no local parent, no api parent: should remove all",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}, "AA": {}},
			localParents: map[string]*v1.Pod{"AA": {}},
			apiParents:   map[string]*v1.Pod{"AA": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
				"AA": {},
			},
			expectRemove: []string{"AA", "kube-system/pod-checkpointer"},
		},
		{
			desc:         "Active pod-checkpointer, no local parent, no api parent: should remove all",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}, "AA": {}},
			localParents: map[string]*v1.Pod{"AA": {}},
			apiParents:   map[string]*v1.Pod{"AA": {}},
			activeCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
				"AA": {},
			},
			expectRemove: []string{"AA", "kube-system/pod-checkpointer"},
		},
		{
			desc:         "Running as an on-disk checkpointer: Inactive pod-checkpointer, local parent, local running, api parent: should start",
			podName:      "pod-checkpointer-mynode",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			localParents: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			apiParents:   map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
			},
			expectStart: []string{"kube-system/pod-checkpointer"},
		},
		{
			desc:         "Running as an on-disk checkpointer: Inactive pod-checkpointer, local parent, no local running, api not reachable: should start",
			podName:      "pod-checkpointer-mynode",
			localParents: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
			},
			expectStart: []string{"kube-system/pod-checkpointer"},
		},
		{
			desc:         "Running as an on-disk checkpointer: Inactive pod-checkpointer, no local parent, no api parent: should remove in the last",
			podName:      "pod-checkpointer-mynode",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}, "AA": {}},
			localParents: map[string]*v1.Pod{"BB": {}},
			apiParents:   map[string]*v1.Pod{"BB": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
				"AA": {},
			},
			expectRemove: []string{"AA", "kube-system/pod-checkpointer"},
		},
		{
			desc:         "Running as an on-disk checkpointer: Inactive pod-checkpointer, no local parent, no api parent: should remove all",
			podName:      "pod-checkpointer-mynode",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}, "AA": {}},
			localParents: map[string]*v1.Pod{"AA": {}},
			apiParents:   map[string]*v1.Pod{"AA": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
				"AA": {},
			},
			expectRemove: []string{"AA", "kube-system/pod-checkpointer"},
		},
		{
			desc:         "Running as an on-disk checkpointer: Active pod-checkpointer, no local parent, no api parent: should remove all",
			podName:      "pod-checkpointer-mynode",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}, "AA": {}},
			localParents: map[string]*v1.Pod{"AA": {}},
			apiParents:   map[string]*v1.Pod{"AA": {}},
			activeCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
				"AA": {},
			},
			expectRemove: []string{"AA", "kube-system/pod-checkpointer"},
		},
	}

	for _, tc := range cases {
		cp := CheckpointerPod{
			NodeName:     "mynode",
			PodName:      "pod-checkpointer",
			PodNamespace: "kube-system",
		}
		if tc.podName != "" {
			cp.PodName = tc.podName
		}
		gotStart, gotStop, gotRemove := process(tc.localRunning, tc.localParents, tc.apiParents, tc.activeCheckpoints, tc.inactiveCheckpoints, cp)
		if !reflect.DeepEqual(tc.expectStart, gotStart) ||
			!reflect.DeepEqual(tc.expectStop, gotStop) ||
			!reflect.DeepEqual(tc.expectRemove, gotRemove) {
			t.Errorf("For test: %s\nExpected start: %s Got: %s\nExpected stop: %s Got: %s\nExpected remove: %s Got: %s\n",
				tc.desc, tc.expectStart, gotStart, tc.expectStop, gotStop, tc.expectRemove, gotRemove)
		}
	}
}
