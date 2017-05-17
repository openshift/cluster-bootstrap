package e2e

import (
	"fmt"

	"k8s.io/client-go/pkg/api/v1"

	"golang.org/x/crypto/ssh"
)

type Node struct {
	apiNode *v1.Node
}

func NewNode(n *v1.Node) *Node {
	return &Node{apiNode: n}
}

func (n *Node) ExternalIP() string {
	var host string
	for _, addr := range n.apiNode.Status.Addresses {
		if addr.Type == v1.NodeExternalIP {
			host = addr.Address
			break
		}
	}
	return host
}

func (n *Node) SSH(cmd string) (stdout, stderr []byte, err error) {
	host := n.ExternalIP()
	if host == "" {
		return nil, nil, fmt.Errorf("cannot find external IP for node %q", n.apiNode.Name)
	}
	return sshClient.SSH(host, cmd)
}

func (n *Node) Reboot() error {
	stdout, stderr, err := n.SSH("sudo reboot")
	if _, ok := err.(*ssh.ExitMissingError); ok {
		// A terminated session is perfectly normal during reboot.
		err = nil
	}

	if err != nil {
		return fmt.Errorf("issuing reboot command failed\nstdout:%s\nstderr:%s", stdout, stderr)
	}
	return nil
}
