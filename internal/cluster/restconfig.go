// Package cluster builds the Kubernetes REST client for kwd.
package cluster

import (
	"fmt"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// LoadRESTConfig builds REST client settings. When kubeconfig is empty, the
// default kubeconfig discovery is tried first, then in-cluster service account
// credentials (kzero-school). context selects a specific kubeconfig context;
// empty means current-context.
func LoadRESTConfig(kubeconfig, context string) (*rest.Config, error) {
	overrides := &clientcmd.ConfigOverrides{}
	if strings.TrimSpace(context) != "" {
		overrides.CurrentContext = strings.TrimSpace(context)
	}

	if k := strings.TrimSpace(kubeconfig); k != "" {
		cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: k},
			overrides,
		).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("load kubeconfig: %w", err)
		}
		return cfg, nil
	}

	loading := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loading, overrides).ClientConfig()
	if err == nil {
		return cfg, nil
	}

	ic, icErr := rest.InClusterConfig()
	if icErr == nil {
		return ic, nil
	}

	return nil, fmt.Errorf("load kubeconfig: %w (in-cluster: %v)", err, icErr)
}

// NewClientset builds a typed clientset from REST config.
func NewClientset(cfg *rest.Config) (*kubernetes.Clientset, error) {
	return kubernetes.NewForConfig(cfg)
}
