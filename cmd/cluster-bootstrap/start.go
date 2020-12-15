package main

import (
	"errors"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/openshift/cluster-bootstrap/pkg/start"
)

var (
	cmdStart = &cobra.Command{
		Use:          "start",
		Short:        "Start the control plane",
		Long:         "",
		PreRunE:      validateStartOpts,
		RunE:         runCmdStart,
		SilenceUsage: true,
	}

	startOpts struct {
		assetDir             string
		podManifestPath      string
		strict               bool
		requiredPodClauses   []string
		waitForTearDownEvent string
		earlyTearDown        bool
		assetsCreatedTimeout time.Duration
	}
)

var defaultRequiredPods = []string{
	"kube-system/pod-checkpointer",
	"kube-system/kube-apiserver",
	"kube-system/kube-scheduler",
	"kube-system/kube-controller-manager",
}

func init() {
	cmdRoot.AddCommand(cmdStart)
	cmdStart.Flags().StringVar(&startOpts.assetDir, "asset-dir", "", "Path to the cluster asset directory.")
	cmdStart.Flags().StringVar(&startOpts.podManifestPath, "pod-manifest-path", "/etc/kubernetes/manifests", "The location where the kubelet is configured to look for static pod manifests.")
	cmdStart.Flags().BoolVar(&startOpts.strict, "strict", false, "Strict mode will cause start command to exit early if any manifests in the asset directory cannot be created.")
	cmdStart.Flags().StringSliceVar(&startOpts.requiredPodClauses, "required-pods", defaultRequiredPods, "List of pods name prefixes with their namespace (written as <namespace>/<pod-prefix>) that are required to be running and ready before the start command does the pivot, or alternatively a list of or'ed pod prefixes with a description (written as <desc>:<namespace>/<pod-prefix>|<namespace>/<pod-prefix>|...).")
	cmdStart.Flags().StringVar(&startOpts.waitForTearDownEvent, "tear-down-event", "", "if this optional event name of the form <ns>/<event-name> is given, the event is waited for before tearing down the bootstrap control plane")
	cmdStart.Flags().BoolVar(&startOpts.earlyTearDown, "tear-down-early", true, "tear down immediate after the non-bootstrap control plane is up and bootstrap-success event is created.")
	cmdStart.Flags().DurationVar(&startOpts.assetsCreatedTimeout, "assets-create-timeout", time.Duration(60)*time.Minute, "how long we wait (in minutes) until the assets must all be created.")
}

func runCmdStart(cmd *cobra.Command, args []string) error {
	podPrefixes, err := parsePodPrefixes(startOpts.requiredPodClauses)
	if err != nil {
		return err
	}

	bk, err := start.NewStartCommand(start.Config{
		AssetDir:             startOpts.assetDir,
		PodManifestPath:      startOpts.podManifestPath,
		Strict:               startOpts.strict,
		RequiredPodPrefixes:  podPrefixes,
		WaitForTearDownEvent: startOpts.waitForTearDownEvent,
		EarlyTearDown:        startOpts.earlyTearDown,
		AssetsCreatedTimeout: startOpts.assetsCreatedTimeout,
	})
	if err != nil {
		return err
	}

	return bk.Run()
}

// parsePodPrefixes parses <ns>/<pod-prefix> or <desc>:<ns>/<pod-prefix>|... into a map with
// the description as key and <ns>/<pod-prefix> as values.
func parsePodPrefixes(clauses []string) (map[string][]string, error) {
	podPrefixes := map[string][]string{}
	for _, p := range clauses {
		if strings.Contains(p, ":") {
			ss := strings.Split(p, ":")
			desc := ss[0]
			ps := strings.Split(ss[1], "|")
			podPrefixes[desc] = append(podPrefixes[desc], ps...)
		} else if strings.Contains(p, "|") {
			return nil, errors.New("required-pods must be either <namespace>/<pod-name> or <desc>:<namespace>/<pod-name>|<namespace>/<pod-name>|...")
		} else {
			podPrefixes[p] = []string{p}
		}
	}
	return podPrefixes, nil
}

func validateStartOpts(cmd *cobra.Command, args []string) error {
	if startOpts.podManifestPath == "" {
		return errors.New("missing required flag: --pod-manifest-path")
	}
	if startOpts.assetDir == "" {
		return errors.New("missing required flag: --asset-dir")
	}
	if _, err := parsePodPrefixes(startOpts.requiredPodClauses); err != nil {
		return err
	}
	return nil
}
