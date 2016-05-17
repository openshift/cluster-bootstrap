package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/coreos/bootkube/pkg/asset"
	"github.com/coreos/bootkube/pkg/tlsutil"
)

var (
	cmdRender = &cobra.Command{
		Use:          "render",
		Short:        "Render default cluster manifests",
		Long:         "",
		PreRunE:      validateRenderOpts,
		RunE:         runCmdRender,
		SilenceUsage: true,
	}

	renderOpts struct {
		assetDir    string
		etcdServers string
		apiServers  string
		altNames    string
	}
)

func init() {
	cmdRoot.AddCommand(cmdRender)
	cmdRender.Flags().StringVar(&renderOpts.assetDir, "asset-dir", "", "Output path for rendered assets")
	cmdRender.Flags().StringVar(&renderOpts.etcdServers, "etcd-servers", "http://127.0.0.1:2379", "List of etcd servers URLs including host:port, comma separated")
	cmdRender.Flags().StringVar(&renderOpts.apiServers, "api-servers", "https://127.0.0.1:443", "List of API server URLs including host:port, commma seprated")
	cmdRender.Flags().StringVar(&renderOpts.altNames, "api-server-alt-names", "", "List of SANs to use in api-server certificate. Example: 'IP=127.0.0.1,IP=127.0.0.2,DNS=localhost'. If empty, SANs will be extracted from the --api-servers flag.")
}

func runCmdRender(cmd *cobra.Command, args []string) error {
	config, err := flagsToAssetConfig()
	if err != nil {
		return err
	}

	as, err := asset.NewDefaultAssets(*config)
	if err != nil {
		return err
	}

	return as.WriteFiles(renderOpts.assetDir)
}

func validateRenderOpts(cmd *cobra.Command, args []string) error {
	if renderOpts.assetDir == "" {
		return errors.New("Missing required flag: --asset-dir")
	}
	if renderOpts.etcdServers == "" {
		return errors.New("Missing required flag: --etcd-servers")
	}
	if renderOpts.apiServers == "" {
		return errors.New("Missing requried flag: --api-servers")
	}
	return nil
}

func flagsToAssetConfig() (c *asset.Config, err error) {
	etcdServers, err := parseURLs(renderOpts.etcdServers)
	if err != nil {
		return nil, err
	}
	apiServers, err := parseURLs(renderOpts.apiServers)
	if err != nil {
		return nil, err
	}
	altNames, err := parseAltNames(renderOpts.altNames)
	if err != nil {
		return nil, err
	}
	if altNames == nil {
		// Fall back to parsing from api-server list
		altNames = altNamesFromURLs(apiServers)
	}
	return &asset.Config{
		EtcdServers: etcdServers,
		APIServers:  apiServers,
		AltNames:    altNames,
	}, nil
}

func parseURLs(s string) ([]*url.URL, error) {
	var out []*url.URL
	for _, u := range strings.Split(s, ",") {
		parsed, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		out = append(out, parsed)
	}
	return out, nil
}

func parseAltNames(s string) (*tlsutil.AltNames, error) {
	if s == "" {
		return nil, nil
	}
	var alt tlsutil.AltNames
	for _, an := range strings.Split(s, ",") {
		switch {
		case strings.HasPrefix(an, "DNS="):
			alt.DNSNames = append(alt.DNSNames, strings.TrimPrefix(an, "DNS="))
		case strings.HasPrefix(an, "IP="):
			ip := net.ParseIP(strings.TrimPrefix(an, "IP="))
			if ip == nil {
				return nil, fmt.Errorf("Invalid IP alt name: %s", an)
			}
			alt.IPs = append(alt.IPs, ip)
		default:
			return nil, fmt.Errorf("Invalid alt name: %s", an)
		}
	}
	return &alt, nil
}

func altNamesFromURLs(urls []*url.URL) *tlsutil.AltNames {
	var an tlsutil.AltNames
	for _, u := range urls {
		host, _, err := net.SplitHostPort(u.Host)
		if err != nil {
			host = u.Host
		}
		ip := net.ParseIP(host)
		if ip == nil {
			an.DNSNames = append(an.DNSNames, host)
		} else {
			an.IPs = append(an.IPs, ip)
		}
	}
	return &an
}
