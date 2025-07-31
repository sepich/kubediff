package diff

import (
	"fmt"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func getClients(opts *Options) (dynamic.Interface, discovery.DiscoveryInterface, error) {
	config, err := buildConfig(opts)
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

func buildConfig(opts *Options) (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if opts.Kubeconfig != "" {
		loadingRules.ExplicitPath = opts.Kubeconfig
	}

	configOverrides := &clientcmd.ConfigOverrides{
		ClusterInfo: clientcmdapi.Cluster{},
		Context: clientcmdapi.Context{
			Namespace: opts.Namespace,
		},
	}

	if opts.Context != "" {
		configOverrides.CurrentContext = opts.Context
	}

	if opts.Cluster != "" {
		configOverrides.Context.Cluster = opts.Cluster
	}

	if opts.Token != "" {
		configOverrides.AuthInfo.Token = opts.Token
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	return clientConfig.ClientConfig()
}
