package collector

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	defaultSSHPort = 22
	defaultSSHUser = "core"
)

func newSSHClient(config *scpConfig) (*ssh.Client, error) {
	var authMethod ssh.AuthMethod
	sock := os.Getenv("SSH_AUTH_SOCK")
	if config.identifyKeyFile != "" {
		key, err := ioutil.ReadFile(config.identifyKeyFile)
		if err != nil {
			return nil, err
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, err
		}

		authMethod = ssh.PublicKeys(signer)
	} else if sock != "" {
		sshAgent, err := net.Dial("unix", sock)
		if err != nil {
			return nil, err
		}

		authMethod = ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)
	} else {
		return nil, fmt.Errorf("no ssh connection authentication provided")
	}

	if config.user == "" {
		config.user = defaultSSHUser
	}
	sshConfig := &ssh.ClientConfig{
		User:            config.user,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	sshConfig.SetDefaults()

	if config.port == 0 {
		config.port = defaultSSHPort
	}
	endpoint := fmt.Sprintf("%s:%d", config.host, config.port)
	client, err := ssh.Dial("tcp", endpoint, sshConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}
