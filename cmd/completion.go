package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

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
			return cmd.Root().GenBashCompletion(cmd.OutOrStdout())
		case "zsh":
			return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
		case "fish":
			return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletion(cmd.OutOrStdout())
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

// loadClusterGVRs loads the GroupVersionResources (GVRs) from the Kubernetes cluster
func loadClusterGVRs(kubeConfigPath string) ([]string, error) {
	const cacheTTL = 60 * time.Second

	cacheKey := strings.ReplaceAll(kubeConfigPath, string(os.PathSeparator), "_")
	cachePath := filepath.Join(os.TempDir(), "loog_complete_"+cacheKey+".json")
	if info, err := os.Stat(cachePath); err == nil && time.Since(info.ModTime()) < cacheTTL {
		if data, err := os.ReadFile(cachePath); err == nil {
			var cached []string
			if json.Unmarshal(data, &cached) == nil {
				return cached, nil
			}
		}
	}

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

	// sort the GVRs for consistent output
	sort.SliceStable(gvrList, func(i, j int) bool {
		return gvrList[i] < gvrList[j]
	})

	// cache the GVRs to a file
	func() {
		data, err := json.Marshal(gvrList)
		if err != nil {
			setupLog.Error().Err(err).Msg("failed to marshal GVRs for caching")
			return
		}
		err = os.WriteFile(cachePath, data, 0o644)
		if err != nil {
			setupLog.Error().Err(err).Msg("failed to write GVRs to cache file")
			return
		}
	}()

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
