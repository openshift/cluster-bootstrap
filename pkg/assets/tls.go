package assets

import (
	"crypto/rsa"
	"crypto/x509"

	"github.com/coreos/bootkube/pkg/tlsutil"
)

func NewTLSAssets() ([]Asset, error) {
	var assets []Asset

	caKey, caCert, err := newCACert()
	if err != nil {
		return assets, err
	}

	apiKey, apiCert, err := newAPIKeyAndCert(caCert, caKey)
	if err != nil {
		return assets, err
	}

	saPrivKey, err := tlsutil.NewPrivateKey()
	if err != nil {
		return assets, err
	}

	saPubKey, err := tlsutil.EncodePublicKeyPEM(&saPrivKey.PublicKey)
	if err != nil {
		return assets, err
	}

	assets = append(assets, []Asset{
		{Name: AssetPathCAKey, Data: tlsutil.EncodePrivateKeyPEM(caKey)},
		{Name: AssetPathCACert, Data: tlsutil.EncodeCertificatePEM(caCert)},
		{Name: AssetPathAPIServerKey, Data: tlsutil.EncodePrivateKeyPEM(apiKey)},
		{Name: AssetPathAPIServerCert, Data: tlsutil.EncodeCertificatePEM(apiCert)},
		{Name: AssetPathServiceAccountPrivKey, Data: tlsutil.EncodePrivateKeyPEM(saPrivKey)},
		{Name: AssetPathServiceAccountPubKey, Data: saPubKey},
	}...)
	return assets, nil
}

func newCACert() (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}

	config := tlsutil.CertConfig{
		CommonName:   "kube-ca",
		Organization: []string{"kube-aws"},
	}

	cert, err := tlsutil.NewSelfSignedCACertificate(config, key)
	if err != nil {
		return nil, nil, err
	}

	return key, cert, err
}

func newAPIKeyAndCert(caCert *x509.Certificate, caKey *rsa.PrivateKey) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}
	config := tlsutil.CertConfig{
		CommonName:   "kube-apiserver",
		Organization: []string{"kube-master"},
		IPAddresses:  []string{"172.17.4.100", "127.0.0.1"}, //TODO(aaron): hack-placeholder
		DNSNames: []string{
			"kubernetes",
			"kubernetes.default",
			"kubernetes.default.svc",
			"kubernetes.default.svc.cluster.local",
		},
	}
	cert, err := tlsutil.NewSignedCertificate(config, key, caCert, caKey)
	if err != nil {
		return nil, nil, err
	}
	return key, cert, err
}
