package cmd

import (
	"fmt"
	"sync"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	cachedGVRs []string
	gvrsOnce   sync.Once
)

var completionCmd = &cobra.Command{
	Use:       "completion [SHELL]",
	Short:     "Prints shell completion scripts",
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			_ = cmd.Root().GenBashCompletion(cmd.OutOrStdout())
		case "zsh":
			_ = cmd.Root().GenZshCompletion(cmd.OutOrStdout())
		case "fish":
			_ = cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
		case "powershell":
			_ = cmd.Root().GenPowerShellCompletion(cmd.OutOrStdout())
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

func loadClusterGVRs(kubeConfigPath string) ([]string, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("building kube config: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %w", err)
	}
	lists, err := cs.Discovery().ServerPreferredResources()
	if err != nil {
		return nil, fmt.Errorf("getting server preferred resources: %w", err)
	}
	var gvrList []string
	for _, list := range lists {
		if len(list.APIResources) == 0 {
			continue
		}
		for _, res := range list.APIResources {
			// only include resources that can be listed or watched
			isValid := false
			for _, verb := range res.Verbs {
				if verb == "list" || verb == "watch" {
					isValid = true
					break
				}
			}
			if !isValid {
				continue
			}
			gvr := fmt.Sprintf("%s/%s", list.GroupVersion, res.Name)
			gvrList = append(gvrList, gvr)
		}
	}
	return gvrList, nil
}

func gvrCompletion(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	gvrsOnce.Do(func() {
		if s, err := loadClusterGVRs(kubeConfigPath); err == nil {
			cachedGVRs = s
		}
	})
	return cachedGVRs, cobra.ShellCompDirectiveNoFileComp
}
