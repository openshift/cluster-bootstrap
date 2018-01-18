// Package collector allows collecting logs from a k8s cluster brought up with bootkube semantics.
//
// It creates following assets:
// 	* fluentd-master deployment on one of the master nodes
// 	* fluentd-worker daemonset to pushes all the logs to fluentd-master
// 	* fluentd-master service for workers to talk to the master
// 	* fluentd-master writes all container logs, docker and kubelet service logs to disk at /var/log/log-collector
//
// For example:
//
// Colecting and writting apiserver logs to local dir:
// 	cr = collector.New(&collector.Config{
// 		K8sClient:     client,
// 		Namespace:     namespace,
// 	})
//
//	 if err := cr.Start(); err != nil {
//		...
//	 }
//
//	 if err := cr.OutputToLocal("/tmp/log-collector"); err != nil {
//	 	...
//	 }
//
//	 results, err := cr.CollectPodLogs("kube-apiserver")
//	 if err != nil {
//	 	...
//	 }
//
//	 if err := cr.Cleanup(); err != nil {
//	 	...
//	 }
//
// Colecting and writting apiserver logs to s3:
//	 cr = collector.New(&collector.Config{
//	 	K8sClient:     client,
//	 	Namespace:     namespace,
//	 })
//
//	 if err := cr.Start(); err != nil {
//	 	...
//	 }
//
//	 if err := cr.OutputToS3(os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), os.Getenv("AWS_REGION"), "log-collector", "prefix"); err != nil {
//	 	...
//	 }
//
//	 results, err := cr.CollectPodLogs("kube-apiserver")
//	 if err != nil {
//	 	...
//	 }
//
//	 if err := cr.Cleanup(); err != nil {
//	 	...
//	 }
//
package collector
