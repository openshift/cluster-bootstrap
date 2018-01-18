package cluster

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Options sets Cluster object options.
type Options func(c *Cluster)

// WithSSHUser defines the user to be used for ssh.
func WithSSHUser(u string) Options {
	return func(c *Cluster) {
		c.sshConfig.User = u
	}
}

// WithSSHPort defines the port to be used for ssh.
func WithSSHPort(p int32) Options {
	return func(c *Cluster) {
		c.sshConfig.Port = p
	}
}

// WithSSHIdentityKeyFile defines the path of the key to be used for ssh.
func WithSSHIdentityKeyFile(path string) Options {
	return func(c *Cluster) {
		c.sshConfig.IdentifyKeyFile = path
	}
}

// WithMaxDisruption defines the no. of parallel nodes rebooting.
func WithMaxDisruption(d interface{}) Options {
	return func(c *Cluster) {
		var dis intstr.IntOrString
		switch d.(type) {
		case int:
			dis = intstr.FromInt(d.(int))
		case string:
			dis = intstr.FromString(d.(string))
		}
		c.MaxDisruption = dis
	}
}
