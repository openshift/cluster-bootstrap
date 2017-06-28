package e2e

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/terminal"
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

// Manhole connects os.Stdin, os.Stdout, and os.Stderr to an interactive shell
// session on the Machine m. Manhole blocks until the shell session has ended.
// If os.Stdin does not refer to a TTY, Manhole returns immediately with a nil
// error. Copied from github.com/coreos/mantle/platform/util.go
func (c *SSHClient) Manhole(host string) error {
	fd := int(os.Stdin.Fd())
	if !terminal.IsTerminal(fd) {
		return nil
	}

	tstate, _ := terminal.MakeRaw(fd)
	defer terminal.Restore(fd, tstate)

	client, err := ssh.Dial("tcp", host+":22", c.ClientConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("SSH session failed: %v", err)
	}

	defer session.Close()

	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	modes := ssh.TerminalModes{
		ssh.TTY_OP_ISPEED: 115200,
		ssh.TTY_OP_OSPEED: 115200,
	}

	cols, lines, err := terminal.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}

	if err = session.RequestPty(os.Getenv("TERM"), lines, cols, modes); err != nil {
		return fmt.Errorf("failed to request pseudo terminal: %s", err)
	}

	if err := session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %s", err)
	}

	if err := session.Wait(); err != nil {
		return fmt.Errorf("failed to wait for session: %s", err)
	}

	return nil
}
