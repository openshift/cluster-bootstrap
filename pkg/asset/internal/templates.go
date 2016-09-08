// Package internal holds asset templates used by bootkube.
package internal

var (
	KubeConfigTemplate = []byte(`apiVersion: v1
kind: Config
clusters:
- name: local
  cluster:
    server: {{ .Server }}
    certificate-authority-data: {{ .CACert }}
users:
- name: kubelet
  user:
    client-certificate-data: {{ .KubeletCert}}
    client-key-data: {{ .KubeletKey }}
contexts:
- context:
    cluster: local
    user: kubelet
`)
	KubeletTemplate = []byte(`apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: kubelet
  namespace: kube-system
  labels:
    k8s-app: kubelet
    version: v1.3.6_coreos.0
spec:
  template:
    metadata:
      labels:
        k8s-app: kubelet
        version: v1.3.6_coreos.0
    spec:
      containers:
      - name: kubelet
        image: quay.io/coreos/hyperkube:v1.3.6_coreos.0
        command:
        - /nsenter
        - --target=1
        - --mount
        - --wd=.
        - --
        - ./hyperkube
        - kubelet
        - --api-servers={{ index .APIServers 0 }}
        - --config=/etc/kubernetes/manifests
        - --allow-privileged
        - --hostname-override=$(MY_POD_IP)
        - --cluster-dns=10.3.0.10
        - --cluster-domain=cluster.local
        - --kubeconfig=/etc/kubernetes/kubeconfig
        - --lock-file=/var/run/lock/kubelet.lock
        - --minimum-container-ttl-duration=3m0s
        env:
          - name: MY_POD_IP
            valueFrom:
              fieldRef:
                fieldPath: status.podIP
        securityContext:
          privileged: true
        volumeMounts:
        - name: dev
          mountPath: /dev
        - name: run
          mountPath: /run
        - name: sys
          mountPath: /sys
          readOnly: true
        - name: etc-kubernetes
          mountPath: /etc/kubernetes
          readOnly: true
        - name: etc-ssl-certs
          mountPath: /etc/ssl/certs
          readOnly: true
        - name: var-lib-docker
          mountPath: /var/lib/docker
        - name: var-lib-kubelet
          mountPath: /var/lib/kubelet
        - name: var-lib-rkt
          mountPath: /var/lib/rkt
      hostNetwork: true
      hostPID: true
      volumes:
      - name: dev
        hostPath:
          path: /dev
      - name: run
        hostPath:
          path: /run
      - name: sys
        hostPath:
          path: /sys
      - name: etc-kubernetes
        hostPath:
          path: /etc/kubernetes
      - name: etc-ssl-certs
        hostPath:
          path: /usr/share/ca-certificates
      - name: var-lib-docker
        hostPath:
          path: /var/lib/docker
      - name: var-lib-kubelet
        hostPath:
          path: /var/lib/kubelet
      - name: var-lib-rkt
        hostPath:
          path: /var/lib/rkt
`)
	APIServerTemplate = []byte(`apiVersion: "extensions/v1beta1"
kind: DaemonSet
metadata:
  name: kube-apiserver
  namespace: kube-system
  labels:
    k8s-app: kube-apiserver
    version: v1.3.6_coreos.0
spec:
  template:
    metadata:
      labels:
        k8s-app: kube-apiserver
        version: v1.3.6_coreos.0
    spec:
      nodeSelector:
        master: "true"
      hostNetwork: true
      containers:
      - name: checkpoint-installer
        image: quay.io/coreos/pod-checkpointer:91d5a311eee40d579a8f0549c10eeed57979d6c4
        command:
        - /checkpoint-installer.sh
        volumeMounts:
        - mountPath: /etc/kubernetes/manifests
          name: etc-k8s-manifests
      - name: kube-apiserver
        image: quay.io/coreos/hyperkube:v1.3.6_coreos.0
        command:
        - /hyperkube
        - apiserver
        - --bind-address=0.0.0.0
        - --secure-port=443
        - --insecure-port=8080
        - --advertise-address=$(MY_POD_IP)
        - --etcd-servers={{ range $i, $e := .EtcdServers }}{{ if $i }},{{end}}{{ $e }}{{end}}
        - --allow-privileged=true
        - --service-cluster-ip-range=10.3.0.0/24
        - --admission-control=NamespaceLifecycle,LimitRanger,ServiceAccount,ResourceQuota
        - --tls-cert-file=/etc/kubernetes/secrets/apiserver.crt
        - --tls-private-key-file=/etc/kubernetes/secrets/apiserver.key
        - --service-account-key-file=/etc/kubernetes/secrets/service-account.pub
        - --client-ca-file=/etc/kubernetes/secrets/ca.crt
        env:
          - name: MY_POD_IP
            valueFrom:
              fieldRef:
                fieldPath: status.podIP
        volumeMounts:
        - mountPath: /etc/ssl/certs
          name: ssl-certs-host
          readOnly: true
        - mountPath: /etc/kubernetes/secrets
          name: secrets
          readOnly: true
      volumes:
      - name: ssl-certs-host
        hostPath:
          path: /usr/share/ca-certificates
      - name: etc-k8s-manifests
        hostPath:
          path: /etc/kubernetes/manifests
      - name: secrets
        secret:
          secretName: kube-apiserver
`)
	ControllerManagerTemplate = []byte(`apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: kube-controller-manager
  namespace: kube-system
  labels:
    k8s-app: kube-controller-manager
    version: v1.3.6_coreos.0
spec:
  template:
    metadata:
      labels:
        k8s-app: kube-controller-manager
        version: v1.3.6_coreos.0
    spec:
      containers:
      - name: kube-controller-manager
        image: quay.io/coreos/hyperkube:v1.3.6_coreos.0
        command:
        - ./hyperkube
        - controller-manager
        - --root-ca-file=/etc/kubernetes/secrets/ca.crt
        - --service-account-private-key-file=/etc/kubernetes/secrets/service-account.key
        - --leader-elect=true
        volumeMounts:
        - name: secrets
          mountPath: /etc/kubernetes/secrets
          readOnly: true
        - name: ssl-host
          mountPath: /etc/ssl/certs
          readOnly: true
      volumes:
      - name: secrets
        secret:
          secretName: kube-controller-manager
      - name: ssl-host
        hostPath:
          path: /usr/share/ca-certificates
`)
	SchedulerTemplate = []byte(`apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: kube-scheduler
  namespace: kube-system
  labels:
    k8s-app: kube-scheduler
    version: v1.3.6_coreos.0
spec:
  template:
    metadata:
      labels:
        k8s-app: kube-scheduler
        version: v1.3.6_coreos.0
    spec:
      containers:
      - name: kube-scheduler
        image: quay.io/coreos/hyperkube:v1.3.6_coreos.0
        command:
        - ./hyperkube
        - scheduler
        - --leader-elect=true
`)
	ProxyTemplate = []byte(`apiVersion: "extensions/v1beta1"
kind: DaemonSet
metadata:
  name: kube-proxy
  namespace: kube-system
  labels:
    k8s_app: kube-proxy
    version: v1.3.6_coreos.0
spec:
  template:
    metadata:
      labels:
        k8s_app: kube-proxy
        version: v1.3.6_coreos.0
    spec:
      hostNetwork: true
      containers:
      - name: kube-proxy
        image: quay.io/coreos/hyperkube:v1.3.6_coreos.0
        command:
        - /hyperkube
        - proxy
        - --master={{ index .APIServers 0 }}
        - --kubeconfig=/etc/kubernetes/kubeconfig
        - --proxy-mode=iptables
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /etc/ssl/certs
          name: ssl-certs-host
          readOnly: true
        - name: etc-kubernetes
          mountPath: /etc/kubernetes
          readOnly: true
      volumes:
      - hostPath:
          path: /usr/share/ca-certificates
        name: ssl-certs-host
      - name: etc-kubernetes
        hostPath:
          path: /etc/kubernetes
`)
	DNSRcTemplate = []byte(`apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: kube-dns-v11
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    version: v11
    kubernetes.io/cluster-service: "true"
spec:
  replicas: 1
  template:
    metadata:
      labels:
        k8s-app: kube-dns
        version: v11
        kubernetes.io/cluster-service: "true"
    spec:
      containers:
      - name: etcd
        image: gcr.io/google_containers/etcd-amd64:2.2.1
        resources:
          limits:
            cpu: 100m
            memory: 500Mi
          requests:
            cpu: 100m
            memory: 50Mi
        command:
        - /usr/local/bin/etcd
        - -data-dir
        - /var/etcd/data
        - -listen-client-urls
        - http://127.0.0.1:2379,http://127.0.0.1:4001
        - -advertise-client-urls
        - http://127.0.0.1:2379,http://127.0.0.1:4001
        - -initial-cluster-token
        - skydns-etcd
        volumeMounts:
        - name: etcd-storage
          mountPath: /var/etcd/data
      - name: kube2sky
        image: gcr.io/google_containers/kube2sky:1.14
        resources:
          limits:
            cpu: 100m
            memory: 200Mi
          requests:
            cpu: 100m
            memory: 50Mi
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 60
          timeoutSeconds: 5
          successThreshold: 1
          failureThreshold: 5
        readinessProbe:
          httpGet:
            path: /readiness
            port: 8081
            scheme: HTTP
          initialDelaySeconds: 30
          timeoutSeconds: 5
        args:
        - --domain=cluster.local
      - name: skydns
        image: gcr.io/google_containers/skydns:2015-10-13-8c72f8c
        resources:
          limits:
            cpu: 100m
            memory: 200Mi
          requests:
            cpu: 100m
            memory: 50Mi
        args:
        - -machines=http://127.0.0.1:4001
        - -addr=0.0.0.0:53
        - -ns-rotate=false
        - -domain=cluster.local.
        ports:
        - containerPort: 53
          name: dns
          protocol: UDP
        - containerPort: 53
          name: dns-tcp
          protocol: TCP
      - name: healthz
        image: gcr.io/google_containers/exechealthz:1.0
        resources:
          limits:
            cpu: 10m
            memory: 20Mi
          requests:
            cpu: 10m
            memory: 20Mi
        args:
        - -cmd=nslookup kubernetes.default.svc.cluster.local 127.0.0.1 >/dev/null
        - -port=8080
        ports:
        - containerPort: 8080
          protocol: TCP
      volumes:
      - name: etcd-storage
        emptyDir: {}
      dnsPolicy: Default
`)
	DNSSvcTemplate = []byte(`apiVersion: v1
kind: Service
metadata:
  name: kube-dns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    kubernetes.io/cluster-service: "true"
    kubernetes.io/name: "KubeDNS"
spec:
  selector:
    k8s-app: kube-dns
  clusterIP:  10.3.0.10
  ports:
  - name: dns
    port: 53
    protocol: UDP
  - name: dns-tcp
    port: 53
    protocol: TCP
`)
	SystemNSTemplate = []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: kube-system
`)
)
