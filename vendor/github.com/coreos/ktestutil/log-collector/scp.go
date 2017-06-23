package collector

import (
	"fmt"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type scpConfig struct {
	user            string
	host            string
	port            int32
	identifyKeyFile string
}

type scp struct {
	conn   *ssh.Client
	client *sftp.Client
}

func newScpClient(config *scpConfig) (*scp, error) {
	conn, err := newSSHClient(config)
	if err != nil {
		return nil, err
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		return nil, err
	}

	return &scp{
		conn:   conn,
		client: client,
	}, nil
}

func (s *scp) Close() {
	s.client.Close()
	s.conn.Close()
}

func (s *scp) GetPodsFilePaths(pod string) ([]string, error) {
	pattern := fmt.Sprintf("/var/log/log-collector/container.*%s*.log", pod)
	return s.client.Glob(pattern)
}

func (s *scp) GetServicesFilePaths(name string) ([]string, error) {
	pattern := fmt.Sprintf("/var/log/log-collector/service.*%s*.log", name)
	return s.client.Glob(pattern)
}

func (s *scp) Open(path string) (*sftp.File, error) {
	return s.client.Open(path)
}
