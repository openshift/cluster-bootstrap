package fluentd

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

func createMasterCfg(client kubernetes.Interface, namespace string) error {
	ci, _, err := api.Codecs.UniversalDecoder().Decode(masterCfg, nil, &v1.ConfigMap{})
	if err != nil {
		return fmt.Errorf("error decoding master configmap: %v", err)
	}
	c, ok := ci.(*v1.ConfigMap)
	if !ok {
		return fmt.Errorf("error expecting v1.ConfigMap, found %T", ci)
	}
	c.ObjectMeta.Namespace = namespace
	_, err = client.CoreV1().ConfigMaps(namespace).Create(c)
	if err != nil {
		return fmt.Errorf("error creating master configmap: %v", err)
	}

	return nil
}

func deleteMasterCfg(client kubernetes.Interface, namespace string) error {
	var err error
	err = client.CoreV1().ConfigMaps(namespace).Delete("fluentd-master", &metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil {
		return fmt.Errorf("error deleting master configmap: %v", err)
	}

	return nil
}

func createMasterDeploy(client kubernetes.Interface, namespace string) error {
	di, _, err := api.Codecs.UniversalDecoder().Decode(masterDeploy, nil, &v1beta1.Deployment{})
	if err != nil {
		return fmt.Errorf("error decoding master deployment: %v", err)
	}
	d, ok := di.(*v1beta1.Deployment)
	if !ok {
		return fmt.Errorf("error expecting v1beta1.Deployment, found %T", di)
	}
	d.ObjectMeta.Namespace = namespace
	_, err = client.ExtensionsV1beta1().Deployments(namespace).Create(d)
	if err != nil {
		return fmt.Errorf("error creating master deployment: %v", err)
	}

	return nil
}

func deleteMasterDeploy(client kubernetes.Interface, namespace string) error {
	var err error
	err = client.ExtensionsV1beta1().Deployments(namespace).Delete("fluentd-master", &metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil {
		return fmt.Errorf("error deleting master deployments: %v", err)
	}

	return nil
}

func createMasterSvc(client kubernetes.Interface, namespace string) error {
	si, _, err := api.Codecs.UniversalDecoder().Decode(masterSvc, nil, &v1.Service{})
	if err != nil {
		return fmt.Errorf("error decoding master service: %v", err)
	}
	s, ok := si.(*v1.Service)
	if !ok {
		return fmt.Errorf("error expecting v1.Service, found %T", si)
	}
	s.ObjectMeta.Namespace = namespace
	_, err = client.CoreV1().Services(namespace).Create(s)
	if err != nil {
		return fmt.Errorf("error creating master service: %v", err)
	}
	return nil
}

func deleteMasterSvc(client kubernetes.Interface, namespace string) error {
	var err error
	err = client.CoreV1().Services(namespace).Delete("fluentd-master", &metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil {
		return fmt.Errorf("error deleting master service: %v", err)
	}

	return nil
}

var propagationPolicy = metav1.DeletePropagationForeground

var masterCfg = []byte(`apiVersion: v1
kind: ConfigMap
data:
  output.conf: |-
    <match kubernetes.**>
      @type forest
      subtype file
      remove_prefix kubernetes
      <template>
        time_slice_format %Y%m%d
        path /var/log/log-collector/container.${tag_parts[1]}.${tag_parts[0]}.*.log
        format json
        append true
        flush_interval 10s
        flush_at_shutdown true
      </template>
    </match>

    <match service.**>
      @type forest
      subtype file
      remove_prefix service
      <template>
        time_slice_format %Y%m%d
        path /var/log/log-collector/service.${tag_parts[1]}.${tag_parts[0]}.*.log
        format json
        append true    
        flush_interval 10s
        flush_at_shutdown true
      </template>
    </match>
  fluent.conf: |-
    This is the root config file, which only includes components of the actual configuration

    # Do not collect fluentd's own logs to avoid infinite loops.
    <match fluent.**>
      @type null
    </match>

    <source>
      @type forward
      port 24224
      bind 0.0.0.0
    </source>
    @include output.conf
metadata:
  name: fluentd-master
`)

var masterDeploy = []byte(`apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: fluentd-master
  labels:
    tier: control-plane
    k8s-app: fluentd-master
spec:
  replicas: 1
  template:
    metadata:
      labels:
        tier: control-plane
        k8s-app: fluentd-master
    spec:
      containers:
      - name: fluentd-master
        image: quay.io/abhinavdahiya/bootkube-fluentd
        env:
        - name: FLUENTD_OPT
          value: --no-supervisor -vv
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
          name: fluentd-master
      nodeSelector:
        node-role.kubernetes.io/master: ""
        log-collector.github.com/fluentd-master: ""
`)

var masterSvc = []byte(`apiVersion: v1
kind: Service
metadata:
  name: fluentd-master
  labels:
    tier: control-plane
    k8s-app: fluentd-master
spec:
  ports:
  - port: 24224
    protocol: TCP
    targetPort: 24224
  selector:
    k8s-app: fluentd-master`)
