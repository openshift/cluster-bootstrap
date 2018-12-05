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
}

type scriptCommand struct {
	scriptPath         string
	releaseImageDigest string
}

type scriptTemplate struct {
	ReleaseImageDigest string
}

func NewScriptCommand(config Config) (*scriptCommand, error) {
	return &scriptCommand{
		scriptPath:         config.ScriptPath,
		releaseImageDigest: config.ReleaseImageDigest,
	}, nil
}

func (c *scriptCommand) Run() error {
	bs, err := ioutil.ReadFile(c.scriptPath)
	if err != nil {
		return err
	}

	tmpl, err := template.New(c.scriptPath).Parse(string(bs))
	if err != nil {
		return err
	}
	err = tmpl.Execute(os.Stdout, scriptTemplate{
		ReleaseImageDigest: c.releaseImageDigest,
	})
	if err != nil {
		return err
	}

	return nil
}
