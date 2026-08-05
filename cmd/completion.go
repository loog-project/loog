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

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"

	"github.com/loog-project/loog/internal/resource"
)

var (
	cachedGVRs []string
	gvrsOnce   sync.Once
)

// loadClusterGVRs loads the GroupVersionResources (GVRs) from the Kubernetes cluster
func loadClusterGVRs(kubeConfigPath string) ([]string, error) {
	const cacheTTL = 60 * time.Second

	cacheKey := strings.ReplaceAll(kubeConfigPath, string(os.PathSeparator), "_")
	if cacheKey == "" {
		if env := os.Getenv("KUBECONFIG"); env != "" {
			cacheKey = strings.ReplaceAll(env, string(os.PathSeparator), "_")
		} else {
			cacheKey = "default"
		}
	}
	cachePath := filepath.Join(os.TempDir(), "loog_complete_"+cacheKey+".json")
	if info, err := os.Stat(cachePath); err == nil && time.Since(info.ModTime()) < cacheTTL {
		if data, err := os.ReadFile(cachePath); err == nil {
			var cached []string
			if json.Unmarshal(data, &cached) == nil {
				return cached, nil
			}
		}
	}

	cfg, err := restConfigForKubeconfig(kubeConfigPath)
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

// loadClusterResourceKinds discovers all watchable resource types from the cluster
// and returns them as []resource.Kind for use by the WatchManager.
// It reuses the same discovery API as loadClusterGVRs but extracts richer metadata.
func loadClusterResourceKinds(kubeConfigPath string) ([]resource.Kind, error) {
	cfg, err := restConfigForKubeconfig(kubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("building kube config: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %w", err)
	}
	lists, err := cs.Discovery().ServerPreferredResources()
	if err != nil {
		// Discovery can return partial results on errors (e.g. metrics-server unavailable).
		// Use whatever we got if lists is non-nil, but surface the partial failure.
		if lists == nil {
			return nil, fmt.Errorf("getting server preferred resources: %w", err)
		}
		log.Warn().Err(err).Msg("Partial resource discovery; some resource types may be missing")
	}

	var kinds []resource.Kind
	seen := make(map[string]bool) // deduplicate by Kind name
	for _, list := range lists {
		if len(list.APIResources) == 0 {
			continue
		}
		// Parse group/version from the list's GroupVersion string
		gv := list.GroupVersion
		for _, res := range list.APIResources {
			// Only include resources that support list or watch
			canWatch := false
			for _, verb := range res.Verbs {
				if verb == "list" || verb == "watch" {
					canWatch = true
					break
				}
			}
			if !canWatch {
				continue
			}
			// Skip sub-resources (e.g. "pods/log", "deployments/scale")
			if strings.Contains(res.Name, "/") {
				continue
			}
			if seen[res.Kind] {
				continue
			}
			seen[res.Kind] = true
			kinds = append(kinds, resource.Kind{
				Kind:       res.Kind,
				APIVersion: gv,
				Resource:   res.Name,
				Namespaced: res.Namespaced,
			})
		}
	}

	sort.Slice(kinds, func(i, j int) bool {
		return kinds[i].Kind < kinds[j].Kind
	})
	return kinds, nil
}
