package fluentd

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

func createWorkerCfg(client kubernetes.Interface, namespace string) error {
	ci, _, err := api.Codecs.UniversalDecoder().Decode(workerCfg, nil, &v1.ConfigMap{})
	if err != nil {
		return fmt.Errorf("error decoding worker configmap: %v", err)
	}
	c, ok := ci.(*v1.ConfigMap)
	if !ok {
		return fmt.Errorf("error expecting v1.ConfigMap, found %T", ci)
	}
	c.ObjectMeta.Namespace = namespace
	_, err = client.CoreV1().ConfigMaps(namespace).Create(c)
	if err != nil {
		return fmt.Errorf("error creating worker configmap: %v", err)
	}

	return nil
}

func deleteWorkerCfg(client kubernetes.Interface, namespace string) error {
	var err error
	err = client.CoreV1().ConfigMaps(namespace).Delete("fluentd-worker", &metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil {
		return fmt.Errorf("error deleting worker configmap: %v", err)
	}

	return nil
}

func createWorkerDs(client kubernetes.Interface, namespace string) error {
	dsi, _, err := api.Codecs.UniversalDecoder().Decode(workerDs, nil, &v1beta1.DaemonSet{})
	if err != nil {
		return fmt.Errorf("error decoding worker daemonset: %v", err)
	}
	ds, ok := dsi.(*v1beta1.DaemonSet)
	if !ok {
		return fmt.Errorf("error expecting v1beta1.DaemonSet, found %T", dsi)
	}
	ds.ObjectMeta.Namespace = namespace
	_, err = client.ExtensionsV1beta1().DaemonSets(namespace).Create(ds)
	if err != nil {
		return fmt.Errorf("error creating worker daemonset: %v", err)
	}

	return nil
}

func deleteWorkerDs(client kubernetes.Interface, namespace string) error {
	var err error
	err = client.ExtensionsV1beta1().DaemonSets(namespace).Delete("fluentd-worker", &metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil {
		return fmt.Errorf("error deleting worker daemonset: %v", err)
	}

	return nil
}

var workerCfg = []byte(`apiVersion: v1
kind: ConfigMap
data:
  output.conf: |-
    <match **>
      @type forward
      buffer_type file
      buffer_path /var/log/fluentd-worker/outward.*.buffer
      flush_interval 10s
      <server>
        name fluentd-master
        host fluentd-master
        port 24224
      </server>
    </match>
  kubernetes.conf: |-
    <source>
      @type tail
      path /var/log/containers/*.log
      pos_file /var/log/fluentd-containers.log.pos
      time_format %Y-%m-%dT%H:%M:%S.%NZ
      tag reform.*
      format json
      read_from_head true
      exclude_path ["/var/log/containers/*fluentd*.log"]
    </source>

    <filter reform.**>
      @type parser
      format /^(?<severity>\w)(?<time>\d{4} [^\s]*)\s+(?<pid>\d+)\s+(?<source>[^ \]]+)\] (?<log>.*)/
      reserve_data true
      suppress_parse_error_log true
      key_name log
    </filter>

    <match reform.**>
      @type record_reformer
      enable_ruby true
      tag "raw.kubernetes.${tag_suffix[4].split('-')[0..-2].join('-')}.#{Socket.gethostname}"
    </match>

    <match raw.kubernetes.**>
      @type copy
      <store>
        @type detect_exceptions

        remove_tag_prefix raw
        message log
        stream stream
        multiline_flush_interval 5
        max_bytes 500000
        max_lines 1000
      </store>
    </match>

  systemd.conf: |-
    <source>
      type systemd
      filters [{ "_SYSTEMD_UNIT": "docker.service" }]
      pos_file /var/log/fluentd-docker.pos
      read_from_head true
      tag "service.docker.#{Socket.gethostname}"
    </source>

    <source>
      type systemd
      filters [{ "_SYSTEMD_UNIT": "kubelet.service" }]
      pos_file /var/log/fluentd-kubelet.pos
      read_from_head true
      tag "service.kubelet.#{Socket.gethostname}"
    </source>

    <filter service.**>
      @type record_transformer
      renew_record
      keep_keys MESSAGE
    </filter>

  fluent.conf: |-
    This is the root config file, which only includes components of the actual configuration

    # Do not collect fluentd's own logs to avoid infinite loops.
    <match fluent.**>
      @type null
    </match>

    @include kubernetes.conf
    @include systemd.conf
    @include output.conf
metadata:
  name: fluentd-worker
`)

var workerDs = []byte(`apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: fluentd-worker
  labels:
    tier: node
    k8s-app: fluentd-worker
spec:
  updateStrategy:
    type: RollingUpdate
  template:
    metadata:
      labels:
        k8s-app: fluentd-worker
    spec:
      containers:
      - name: fluentd-worker
        image: quay.io/abhinavdahiya/bootkube-fluentd
        env:
        - name: FLUENTD_OPT
          value: --no-supervisor -vv
        resources:
          limits:
            memory: 300Mi
          requests:
            cpu: 100m
            memory: 200Mi
        volumeMounts:
        - name: varlog
          mountPath: /var/log
        - name: varlibdockercontainers
          mountPath: /var/lib/docker/containers
          readOnly: true
        - name: libsystemddir
          mountPath: /host/lib
          readOnly: true
        - name: config-volume
          mountPath: /fluentd/etc/
      terminationGracePeriodSeconds: 30
      tolerations:
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      volumes:
      - name: varlog
        hostPath:
          path: /var/log
      - name: varlibdockercontainers
        hostPath:
          path: /var/lib/docker/containers
      - name: libsystemddir
        hostPath:
          path: /usr/lib64
      - name: config-volume
        configMap:
          name: fluentd-worker

`)
