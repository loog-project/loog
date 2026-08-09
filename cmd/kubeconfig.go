package cmd

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// restConfigForKubeconfig builds a *rest.Config the same way kubectl resolves
// its kubeconfig, in this order of precedence:
//
//  1. an explicit path (the --kubeconfig flag), when non-empty;
//  2. the KUBECONFIG environment variable, which may list several files to
//     merge (os.PathListSeparator-separated), like kubectl;
//  3. the default ~/.kube/config.
//
// contextName, when non-empty, overrides the current-context (the --context
// flag), matching `kubectl --context`. Passing empty strings yields the
// KUBECONFIG / ~/.kube/config file and its current-context.
func restConfigForKubeconfig(explicitPath, contextName string) (*rest.Config, error) {
	// NewDefaultClientConfigLoadingRules reads KUBECONFIG (a precedence list of
	// files to merge) and falls back to RecommendedHomeFile (~/.kube/config).
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if explicitPath != "" {
		// An explicit --kubeconfig takes priority over KUBECONFIG and the
		// default file, matching `kubectl --kubeconfig`.
		rules.ExplicitPath = explicitPath
	}
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		rules, overrides,
	).ClientConfig()
}
