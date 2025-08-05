package diff

import (
	"errors"
	"fmt"
	"net"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func (d *Diff) getClients() (dynamic.Interface, discovery.DiscoveryInterface, error) {
	config, err := d.buildConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build config: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create discovery client: %w", err)
	}
	return dynamicClient, discoveryClient, nil
}

func (d *Diff) buildConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if d.Kubeconfig != "" {
		loadingRules.ExplicitPath = d.Kubeconfig
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	if d.Namespace != "" {
		configOverrides.Context.Namespace = d.Namespace
	}

	if d.Context != "" {
		configOverrides.CurrentContext = d.Context
	}

	if d.Cluster != "" {
		configOverrides.Context.Cluster = d.Cluster
	}

	if d.Token != "" {
		configOverrides.AuthInfo.Token = d.Token
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	if d.Namespace == "" {
		d.Namespace, _, _ = clientConfig.Namespace()
	}
	return clientConfig.ClientConfig()
}

func isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	if k8serr.IsServerTimeout(err) || k8serr.IsServiceUnavailable(err) || k8serr.IsTimeout(err) {
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Timeout() || opErr.Temporary() {
			return true
		}
	}
	return false
}
