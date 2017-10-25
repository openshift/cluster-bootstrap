package checkpoint

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/golang/glog"
	"k8s.io/client-go/pkg/api/v1"
)

// process() makes decisions on which checkpoints need to be started, stopped, or removed.
// It makes this decision based on inspecting the states from kubelet, apiserver, active/inactive checkpoints.
//
// - localRunningPods: running pods retrieved from CRI shim. Minimal amount of info (no podStatus) as it is extracted from container runtime.
// - localParentPods: pod state from kubelet api for all "to be checkpointed" pods - podStatus may be stale (only as recent as last apiserver contact)
// - apiParentPods: pod state from the api server for all "to be checkpointed" pods
// - activeCheckpoints: checkpoint pod manifests which are currently active & in the static pod manifest
// - inactiveCheckpoints: checkpoint pod manifest which are stored in an inactive directory, but are ready to be activated
//
// The return values are checkpoints which should be started or stopped, and checkpoints which need to be removed altogether.
// The removal of a checkpoint means its parent is no longer scheduled to this node, and we need to GC active / inactive
// checkpoints as well as any secrets / configMaps which are no longer necessary.
func process(localRunningPods, localParentPods, apiParentPods, activeCheckpoints, inactiveCheckpoints map[string]*v1.Pod, checkpointerPod CheckpointerPod) (start, stop, remove []string) {
	// If this variable is filled, then it means we need to remove the pod-checkpointer's checkpoint.
	// We treat the pod-checkpointer's checkpoint specially because we want to put it at the end of
	// the remove queue.
	var podCheckpointerID string

	// We can only make some GC decisions if we've successfully contacted an apiserver.
	// When apiParentPods == nil, that means we were not able to get an updated list of pods.
	removeMap := make(map[string]struct{})
	if len(apiParentPods) != 0 {

		// Scan for inacive checkpoints we should GC
		for id := range inactiveCheckpoints {
			// If the inactive checkpoint still has a parent pod, do nothing.
			// This means the kubelet thinks it should still be running, which has the same scheduling info that we do --
			// so we won't make any decisions about its checkpoint.
			// TODO(aaron): This is a safety check, and may not be necessary -- question is do we trust that the api state we received
			//              is accurate -- and that we should ignore our local state (or assume it could be inaccurate). For example,
			//              local kubelet pod state will be innacurate in the case that we can't contact apiserver (kubelet only keeps
			//              cached responses from api) -- however, we're assuming we've been able to contact api, so this likely is moot.
			if _, ok := localParentPods[id]; ok {
				glog.V(4).Infof("API GC: skipping inactive checkpoint %s", id)
				continue
			}

			// If the inactive checkpoint does not have a parent in the api-server, we must assume it should no longer be running on this node.
			// NOTE: It's possible that a replacement for this pod has not been rescheduled elsewhere, but that's not something we can base our decision on.
			//       For example, if a single scheduler is running, and the node is drained, the scheduler pod will be deleted and there will be no replacement.
			//       However, we don't know this, and as far as the checkpointer is concerned - that pod is no longer scheduled to this node.
			if _, ok := apiParentPods[id]; !ok {
				glog.V(4).Infof("API GC: should remove inactive checkpoint %s", id)

				removeMap[id] = struct{}{}
				if isPodCheckpointer(inactiveCheckpoints[id], checkpointerPod) {
					podCheckpointerID = id
					break
				}

				delete(inactiveCheckpoints, id)
			}
		}

		// Scan active checkpoints we should GC
		for id := range activeCheckpoints {
			// If the active checkpoint does not have a parent in the api-server, we must assume it should no longer be running on this node.
			if _, ok := apiParentPods[id]; !ok {
				glog.V(4).Infof("API GC: should remove active checkpoint %s", id)

				removeMap[id] = struct{}{}
				if isPodCheckpointer(activeCheckpoints[id], checkpointerPod) {
					podCheckpointerID = id
					break
				}

				delete(activeCheckpoints, id)
			}
		}
	}

	// Remove all checkpoints if we need to remove the pod checkpointer itself.
	if podCheckpointerID != "" {
		glog.V(4).Info("Pod checkpointer is removed, removing all checkpoints")
		for id := range inactiveCheckpoints {
			removeMap[id] = struct{}{}
			delete(inactiveCheckpoints, id)
		}
		for id := range activeCheckpoints {
			removeMap[id] = struct{}{}
			delete(activeCheckpoints, id)
		}
	}

	// Can make decisions about starting/stopping checkpoints just with local state.
	//
	// If there is an inactive checkpoint, and no parent pod is running, or the checkpoint
	// is the pod-checkpointer, start the checkpoint.
	for id := range inactiveCheckpoints {
		_, ok := localRunningPods[id]
		if !ok || isPodCheckpointer(inactiveCheckpoints[id], checkpointerPod) {
			glog.V(4).Infof("Should start checkpoint %s", id)
			start = append(start, id)
		}
	}

	// If there is an active checkpoint and a running parent pod, stop the active checkpoint
	// unless this is the pod-checkpointer.
	// The parent may not be in a running state, but the kubelet is trying to start it
	// so we should get out of the way.
	for id := range activeCheckpoints {
		_, ok := localRunningPods[id]
		if ok && !isPodCheckpointer(activeCheckpoints[id], checkpointerPod) {
			glog.V(4).Infof("Should stop checkpoint %s", id)
			stop = append(stop, id)
		}
	}

	// De-duped checkpoints to remove. If we decide to GC a checkpoint, we will clean up both inactive/active.
	for k := range removeMap {
		if k == podCheckpointerID {
			continue
		}
		remove = append(remove, k)
	}
	// Put pod checkpoint at the last of the queue.
	if podCheckpointerID != "" {
		remove = append(remove, podCheckpointerID)
	}

	return start, stop, remove
}

// createCheckpointsForValidParents will iterate through pods which are candidates for checkpointing, then:
// - checkpoint any remote assets they need (e.g. secrets, configmaps)
// - sanitize their podSpec, removing unnecessary information
// - store the manifest on disk in an "inactive" checkpoint location
func (c *checkpointer) createCheckpointsForValidParents(pods map[string]*v1.Pod) {

	needsCheckpointUpdate := lastCheckpoint.IsZero() || time.Since(lastCheckpoint) >= defaultCheckpointTimeout

	for _, pod := range pods {
		id := podFullName(pod)

		cp, err := copyPod(pod)
		if err != nil {
			glog.Errorf("Failed to create checkpoint pod copy for %s: %v", id, err)
			continue
		}

		cp, err = sanitizeCheckpointPod(cp)
		if err != nil {
			glog.Errorf("Failed to sanitize manifest for %s: %v", id, err)
			continue
		}

		podChanged, err := writeCheckpointManifest(cp)
		if err != nil {
			glog.Errorf("Failed to write checkpoint for %s: %v", id, err)
			continue
		}

		// Check for secret and configmap changes if the pods have change or they haven't been checked in a while
		if podChanged || needsCheckpointUpdate {

			_, err = c.checkpointSecretVolumes(pod)
			if err != nil {
				//TODO(aaron): This can end up spamming logs at times when api-server is unavailable. To reduce spam
				//             we could only log error if api-server can't be contacted and existing secret doesn't exist.
				glog.Errorf("Failed to checkpoint secrets for pod %s: %v", id, err)
				continue
			}

			_, err = c.checkpointConfigMapVolumes(pod)
			if err != nil {
				//TODO(aaron): This can end up spamming logs at times when api-server is unavailable. To reduce spam
				//             we could only log error if api-server can't be contacted and existing configmap doesn't exist.
				glog.Errorf("Failed to checkpoint configMaps for pod %s: %v", id, err)
				continue
			}
		}
	}

	// If the secrets/manifests were checked update the lastCheckpoint
	if needsCheckpointUpdate {
		lastCheckpoint = time.Now()
	}
}

func handleRemove(remove []string) {
	for _, id := range remove {
		glog.Infof("Removing checkpoint of: %s", id)

		// Remove Secrets
		p := podFullNameToSecretPath(id)
		if err := os.RemoveAll(p); err != nil {
			glog.Errorf("Failed to remove pod secrets from %s: %s", p, err)
		}

		// Remove ConfipMaps
		p = podFullNameToConfigMapPath(id)
		if err := os.RemoveAll(p); err != nil {
			glog.Errorf("Failed to remove pod configMaps from %s: %s", p, err)
		}

		// Remove inactive checkpoints
		p = podFullNameToInactiveCheckpointPath(id)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			glog.Errorf("Failed to remove inactive checkpoint %s: %v", p, err)
			continue
		}

		// Remove active checkpoints.
		// We do this as the last step because we want to clean up
		// resources before the checkpointer itself exits.
		//
		// TODO(yifan): Removing the pods after removing the secrets/configmaps
		// might disturb other pods since they might want to use the configmap
		// or secrets during termination.
		//
		// However, since we are not waiting for them to terminate anyway, so it's
		// ok to just leave as is for now. We can handle this more gracefully later.
		p = podFullNameToActiveCheckpointPath(id)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			glog.Errorf("Failed to remove active checkpoint %s: %v", p, err)
			continue
		}
	}
}

func handleStop(stop []string) {
	for _, id := range stop {
		glog.Infof("Stopping active checkpoint: %s", id)
		p := podFullNameToActiveCheckpointPath(id)
		if err := os.Remove(p); err != nil {
			if os.IsNotExist(err) { // Sanity check (it's fine - just want to surface this if it's occurring)
				glog.Warningf("Attempted to remove active checkpoint, but manifest no longer exists: %s", p)
			} else {
				glog.Errorf("Failed to stop active checkpoint %s: %v", p, err)
			}
		}
	}
}

func handleStart(start []string) {
	for _, id := range start {
		src := podFullNameToInactiveCheckpointPath(id)
		data, err := ioutil.ReadFile(src)
		if err != nil {
			glog.Errorf("Failed to read checkpoint source: %v", err)
			continue
		}

		dst := podFullNameToActiveCheckpointPath(id)
		if _, err := writeManifestIfDifferent(dst, id, data); err != nil {
			glog.Errorf("Failed to write active checkpoint manifest: %v", err)
		}
	}
}
