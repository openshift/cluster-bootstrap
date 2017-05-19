package e2e

import (
	"bytes"
	"io/ioutil"
	"log"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type SSHClient struct {
	*ssh.ClientConfig
}

// newSSHClientOrDie tries to create an ssh client.
// If $SSH_AUTH_SOCK is set, the use the ssh agent to create the client,
// otherwise read the private key directly.
func newSSHClientOrDie(keypath string) *SSHClient {
	var authMethod ssh.AuthMethod

	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock != "" {
		log.Println("Creating ssh client with ssh agent")
		sshAgent, err := net.Dial("unix", sock)
		if err != nil {
			panic(err)
		}

		authMethod = ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)
	} else {
		log.Println("Creating ssh client with private key")
		key, err := ioutil.ReadFile(keypath)
		if err != nil {
			panic(err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			panic(err)
		}

		authMethod = ssh.PublicKeys(signer)
	}

	sshConfig := &ssh.ClientConfig{
		User:            "core", // TODO(yifan): Assume all nodes are container linux nodes for now.
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	return &SSHClient{sshConfig}
}

func (c *SSHClient) SSH(host, cmd string) (stdout, stderr []byte, err error) {
	client, err := ssh.Dial("tcp", host+":22", c.ClientConfig) // TODO(yifan): Assume all nodes are listening on :22 for ssh requests for now.
	if err != nil {
		return nil, nil, err
	}
	defer client.Conn.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, nil, err
	}
	defer session.Close()

	outBuf := bytes.NewBuffer(nil)
	errBuf := bytes.NewBuffer(nil)
	session.Stdout = outBuf
	session.Stderr = errBuf

	err = session.Run(cmd)

	stdout = bytes.TrimSpace(outBuf.Bytes())
	stderr = bytes.TrimSpace(errBuf.Bytes())

	return stdout, stderr, err
}
