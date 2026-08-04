package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// writeKubeconfig writes a minimal single-cluster kubeconfig pointing at server
// and returns its path.
func writeKubeconfig(t *testing.T, dir, name, ctx, server string) string {
	t.Helper()
	content := "" +
		"apiVersion: v1\n" +
		"kind: Config\n" +
		"clusters:\n" +
		"- name: " + ctx + "\n" +
		"  cluster:\n" +
		"    server: " + server + "\n" +
		"contexts:\n" +
		"- name: " + ctx + "\n" +
		"  context:\n" +
		"    cluster: " + ctx + "\n" +
		"    user: " + ctx + "\n" +
		"current-context: " + ctx + "\n" +
		"users:\n" +
		"- name: " + ctx + "\n" +
		"  user: {}\n"
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	return p
}

func TestRestConfigForKubeconfig_RespectsKUBECONFIG(t *testing.T) {
	dir := t.TempDir()
	fileA := writeKubeconfig(t, dir, "a.yaml", "a", "https://a.example:6443")

	t.Setenv("KUBECONFIG", fileA)

	cfg, err := restConfigForKubeconfig("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Host != "https://a.example:6443" {
		t.Errorf("expected KUBECONFIG cluster host, got %q", cfg.Host)
	}
}

func TestRestConfigForKubeconfig_ExplicitOverridesKUBECONFIG(t *testing.T) {
	dir := t.TempDir()
	fileA := writeKubeconfig(t, dir, "a.yaml", "a", "https://a.example:6443")
	fileB := writeKubeconfig(t, dir, "b.yaml", "b", "https://b.example:6443")

	t.Setenv("KUBECONFIG", fileA)

	// --kubeconfig must win over KUBECONFIG, like `kubectl --kubeconfig`.
	cfg, err := restConfigForKubeconfig(fileB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Host != "https://b.example:6443" {
		t.Errorf("explicit --kubeconfig should win; got host %q", cfg.Host)
	}
}

func TestRestConfigForKubeconfig_MergesKUBECONFIGList(t *testing.T) {
	dir := t.TempDir()
	fileA := writeKubeconfig(t, dir, "a.yaml", "a", "https://a.example:6443")
	fileB := writeKubeconfig(t, dir, "b.yaml", "b", "https://b.example:6443")

	// A colon/semicolon-separated list is merged; current-context comes from
	// the first file that sets it (file A here), matching kubectl.
	t.Setenv("KUBECONFIG", fileA+string(os.PathListSeparator)+fileB)

	cfg, err := restConfigForKubeconfig("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Host != "https://a.example:6443" {
		t.Errorf("merged list should use first file's current-context; got %q", cfg.Host)
	}
}
