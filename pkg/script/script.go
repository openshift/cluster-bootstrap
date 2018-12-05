package script

import (
	"io/ioutil"
	"os"
	"text/template"
)

type Config struct {
	ScriptPath         string
	ReleaseImageDigest string
	EtcdCluster        string
	AssetsDir       string
}

type scriptCommand struct {
	config Config
}

func NewScriptCommand(config Config) (*scriptCommand, error) {
	return &scriptCommand{
		config: config,
	}, nil
}

func (c *scriptCommand) Run() error {
	bs, err := ioutil.ReadFile(c.config.ScriptPath)
	if err != nil {
		return err
	}

	tmpl, err := template.New(c.config.ScriptPath).Parse(string(bs))
	if err != nil {
		return err
	}
	err = tmpl.Execute(os.Stdout, c.config)
	if err != nil {
		return err
	}

	return nil
}
