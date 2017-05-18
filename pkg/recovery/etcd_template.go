package recovery

var RecoveryEtcdTemplate = []byte(`apiVersion: v1
kind: Pod
metadata:
  name: recovery-etcd
  namespace: kube-system
  labels:
    k8s-app: recovery-etcd
spec:
  initContainers:
  - name: recovery
    image: {{ .Image }}
    command: 
    - /usr/local/bin/etcdctl
    - snapshot
    - restore
    - --data-dir=/var/etcd/recovery
    - --name=recovery-etcd
    - --initial-cluster=recovery-etcd=http://localhost:32380
    - --initial-cluster-token=bootkube-recovery
    - --initial-advertise-peer-urls=http://localhost:32380
    - --skip-hash-check=true
    - /var/etcd-backupdir/{{ .BackupFile }}
    env:
    - name: ETCDCTL_API
      value: "3"
    volumeMounts:
      - mountPath: /var/etcd
        name: etcd
        readOnly: false
      - mountPath: /var/etcd-backupdir
        name: etcdbackup
        readOnly: false
  containers:
  - name: etcd
    image: {{ .Image }}
    command:
    - /usr/local/bin/etcd
    - --name=recovery-etcd
    - --listen-client-urls=http://0.0.0.0:32379
    - --listen-peer-urls=http://0.0.0.0:32380
    - --advertise-client-urls=http://localhost:32379
    - --data-dir=/var/etcd/recovery
    volumeMounts:
      - mountPath: /var/etcd
        name: etcd
        readOnly: false
  hostNetwork: true
  restartPolicy: Never
  volumes:
    - name: etcd
      emptyDir: {}
    - name: etcdbackup
      hostPath:
        path: {{ .BackupDir }}
`)
