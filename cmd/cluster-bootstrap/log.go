package main

import (
	"flag"
	"log"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
)

type GlogWriter struct{}

func init() {
	flag.Set("logtostderr", "true")
}

func (writer GlogWriter) Write(data []byte) (n int, err error) {
	klog.Info(string(data))
	return len(data), nil
}

func InitLogs() {
	log.SetOutput(GlogWriter{})
	log.SetFlags(log.LUTC | log.Ldate | log.Ltime)
	flushFreq := 5 * time.Second
	go wait.Until(klog.Flush, flushFreq, wait.NeverStop)
}

func FlushLogs() {
	klog.Flush()
}
