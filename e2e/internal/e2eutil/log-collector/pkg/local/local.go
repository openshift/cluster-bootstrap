package local

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Config defines configuration for local output.
type Config struct {
	// Directory where logs will be written to.
	Dir string
}

// Local implements Collector.Ouput Interface.
// Local allows the logs to be written to local disk.
type Local struct {
	dir string
}

// New returns *Local.
func New(c *Config) (*Local, error) {
	if c.Dir == "" {
		return nil, fmt.Errorf("empty destination directory")
	}

	dir, _ := filepath.Abs(c.Dir)
	return &Local{
		dir: dir,
	}, nil
}

// Put stores the data from f io.ReadSeeker to dir (specified in Local config)
// and dst filename.
// It returns the absolute path of the log file on the disk.
func (l *Local) Put(f io.ReadSeeker, dst string) (string, error) {
	r := bufio.NewReader(f)

	path := filepath.Join(l.dir, dst)
	df, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer df.Close()

	_, err = r.WriteTo(df)
	if err != nil {
		return "", err
	}

	return path, nil
}
