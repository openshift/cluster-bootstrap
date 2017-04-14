package asset

import (
	"crypto/rsa"
	"crypto/x509"
	"net"
	"net/url"
	"strings"

	"github.com/kubernetes-incubator/bootkube/pkg/tlsutil"
)

func newTLSAssets(caCert *x509.Certificate, caPrivKey *rsa.PrivateKey, altNames tlsutil.AltNames) ([]Asset, error) {
	var (
		assets []Asset
		err    error
	)

	apiKey, apiCert, err := newAPIKeyAndCert(caCert, caPrivKey, altNames)
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

	kubeletKey, kubeletCert, err := newKubeletKeyAndCert(caCert, caPrivKey)
	if err != nil {
		return assets, err
	}

	assets = append(assets, []Asset{
		{Name: AssetPathCAKey, Data: tlsutil.EncodePrivateKeyPEM(caPrivKey)},
		{Name: AssetPathCACert, Data: tlsutil.EncodeCertificatePEM(caCert)},
		{Name: AssetPathAPIServerKey, Data: tlsutil.EncodePrivateKeyPEM(apiKey)},
		{Name: AssetPathAPIServerCert, Data: tlsutil.EncodeCertificatePEM(apiCert)},
		{Name: AssetPathServiceAccountPrivKey, Data: tlsutil.EncodePrivateKeyPEM(saPrivKey)},
		{Name: AssetPathServiceAccountPubKey, Data: saPubKey},
		{Name: AssetPathKubeletKey, Data: tlsutil.EncodePrivateKeyPEM(kubeletKey)},
		{Name: AssetPathKubeletCert, Data: tlsutil.EncodeCertificatePEM(kubeletCert)},
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
		Organization: []string{"bootkube"},
	}

	cert, err := tlsutil.NewSelfSignedCACertificate(config, key)
	if err != nil {
		return nil, nil, err
	}

	return key, cert, err
}

func newAPIKeyAndCert(caCert *x509.Certificate, caPrivKey *rsa.PrivateKey, altNames tlsutil.AltNames) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}
	altNames.DNSNames = append(altNames.DNSNames, []string{
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		"kubernetes.default.svc.cluster.local",
	}...)

	config := tlsutil.CertConfig{
		CommonName:   "kube-apiserver",
		Organization: []string{"kube-master"},
		AltNames:     altNames,
	}
	cert, err := tlsutil.NewSignedCertificate(config, key, caCert, caPrivKey)
	if err != nil {
		return nil, nil, err
	}
	return key, cert, err
}

func newKubeletKeyAndCert(caCert *x509.Certificate, caPrivKey *rsa.PrivateKey) (*rsa.PrivateKey, *x509.Certificate, error) {
	// TLS organizations map to Kubernetes groups, and "system:masters"
	// is a well-known Kubernetes group that gives a user admin power.
	//
	// For now, put the kubelets in this group. Later we can restrict
	// their credentials, likely with the help of TLS bootstrapping.
	const orgSystemMasters = "system:masters"

	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}
	config := tlsutil.CertConfig{
		CommonName:   "kubelet",
		Organization: []string{orgSystemMasters},
	}
	cert, err := tlsutil.NewSignedCertificate(config, key, caCert, caPrivKey)
	if err != nil {
		return nil, nil, err
	}
	return key, cert, err
}

func newEtcdTLSAssets(etcdCACert, etcdServerCert *x509.Certificate, etcdServerKey *rsa.PrivateKey, caCert *x509.Certificate, caPrivKey *rsa.PrivateKey, etcdServers []*url.URL) ([]Asset, error) {
	if etcdCACert == nil {
		var err error
		etcdServerKey, etcdServerCert, err = newEtcdServerKeyAndCert(caCert, caPrivKey, etcdServers)
		if err != nil {
			return nil, err
		}
		etcdCACert = caCert
	}

	return []Asset{
		{Name: AssetPathEtcdCA, Data: tlsutil.EncodeCertificatePEM(etcdCACert)},
		{Name: AssetPathEtcdServerKey, Data: tlsutil.EncodePrivateKeyPEM(etcdServerKey)},
		{Name: AssetPathEtcdServerCert, Data: tlsutil.EncodeCertificatePEM(etcdServerCert)},
	}, nil
}

func newEtcdServerKeyAndCert(caCert *x509.Certificate, caPrivKey *rsa.PrivateKey, etcdServers []*url.URL) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}
	var altNames tlsutil.AltNames
	for _, etcdServer := range etcdServers {
		hostname := stripPort(etcdServer.Host)
		if ip := net.ParseIP(hostname); ip != nil {
			altNames.IPs = append(altNames.IPs, ip)
		} else {
			altNames.DNSNames = append(altNames.DNSNames, hostname)
		}
	}
	config := tlsutil.CertConfig{
		CommonName:   "etcd-server",
		Organization: []string{"etcd"},
		AltNames:     altNames,
	}
	cert, err := tlsutil.NewSignedCertificate(config, key, caCert, caPrivKey)
	if err != nil {
		return nil, nil, err
	}
	return key, cert, err
}

// TODO(diegs): remove this and switch to URL.Hostname() once bootkube uses Go 1.8.
func stripPort(hostport string) string {
	colon := strings.IndexByte(hostport, ':')
	if colon == -1 {
		return hostport
	}
	if i := strings.IndexByte(hostport, ']'); i != -1 {
		return strings.TrimPrefix(hostport[:i], "[")
	}
	return hostport[:colon]
}
