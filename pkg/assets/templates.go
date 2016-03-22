package assets

import (
	"bytes"
	"encoding/base64"
	"path/filepath"
	"text/template"

	"github.com/ghodss/yaml"

	"github.com/coreos/bootkube/pkg/assets/internal"
)

var (
	secretNamespace            = "kube-system"
	secretAPIServerName        = "kube-apiserver"
	secretServiceAccountPubKey = "service-account.pub"
	secretTokenFile            = "token-auth.csv"
)

func NewKubeConfig(assets Assets) (Asset, error) {
	var config Asset

	caCert, err := assets.Get(AssetPathCACert)
	if err != nil {
		return config, err
	}

	tmpl, err := template.New(AssetPathKubeConfig).Parse(string(internal.KubeConfigTemplate))
	if err != nil {
		return config, err
	}

	tmplConfig := struct {
		Server string
		Token  string
		CACert string
	}{
		//TODO(aaron): temporary hack. Get token info from generated asset & server from cli opt
		"https://172.17.4.100",
		"token",
		base64.StdEncoding.EncodeToString(caCert.Data),
	}

	var data bytes.Buffer
	if err := tmpl.Execute(&data, tmplConfig); err != nil {
		return config, err
	}

	config.Name = AssetPathKubeConfig
	config.Data = data.Bytes()

	return config, nil
}

func NewAPIServerSecret(assets Assets) (Asset, error) {
	var secret Asset
	data := make(map[string][]byte)

	// API Private Key
	apiKey, err := assets.Get(AssetPathAPIServerKey)
	if err != nil {
		return secret, err
	}
	data[filepath.Base(apiKey.Name)] = apiKey.Data

	// API Cert
	apiCert, err := assets.Get(AssetPathAPIServerCert)
	if err != nil {
		return secret, err
	}
	data[filepath.Base(apiCert.Name)] = apiCert.Data

	// Service account pub-key
	saPubKey, err := assets.Get(AssetPathServiceAccountPubKey)
	if err != nil {
		return secret, err
	}
	data[secretServiceAccountPubKey] = saPubKey.Data

	// Token auth
	tokenAuth, err := assets.Get(AssetPathTokenAuth)
	if err != nil {
		return secret, err
	}
	data[secretTokenFile] = tokenAuth.Data

	y, err := newSecretYaml(secretAPIServerName, secretNamespace, data)
	if err != nil {
		return secret, err
	}

	secret.Name = AssetPathAPIServerSecret
	secret.Data = y

	return secret, nil
}

func NewTokenAuth() Asset {
	// TODO(aaron): temp hack / should at minimum generate random token
	return Asset{
		Name: AssetPathTokenAuth,
		Data: []byte("token,admin,1"),
	}
}

// TODO(aaron): use actual secret object (need to wrap in apiversion/type)
type secret struct {
	ApiVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Metadata   map[string]string `json:"metadata"`
	Type       string            `json:"type"`
	Data       map[string]string `json:"data"`
}

func newSecretYaml(name, namespace string, data map[string][]byte) ([]byte, error) {
	b64data := make(map[string]string)
	for k, v := range data {
		b64data[k] = base64.StdEncoding.EncodeToString(v)
	}
	s := secret{
		ApiVersion: "v1",
		Kind:       "Secret",
		Type:       "Opaque",
		Metadata: map[string]string{
			"name":      name,
			"namespace": namespace,
		},
		Data: b64data,
	}
	return yaml.Marshal(s)
}
