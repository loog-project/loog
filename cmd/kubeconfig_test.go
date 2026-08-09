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

	cfg, err := restConfigForKubeconfig("", "")
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
	cfg, err := restConfigForKubeconfig(fileB, "")
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

	cfg, err := restConfigForKubeconfig("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Host != "https://a.example:6443" {
		t.Errorf("merged list should use first file's current-context; got %q", cfg.Host)
	}
}

// writeTwoContextKubeconfig writes a kubeconfig with two contexts (a -> serverA,
// b -> serverB) whose current-context is "a", and returns its path.
func writeTwoContextKubeconfig(t *testing.T, dir, name, serverA, serverB string) string {
	t.Helper()
	content := "" +
		"apiVersion: v1\n" +
		"kind: Config\n" +
		"clusters:\n" +
		"- name: a\n  cluster:\n    server: " + serverA + "\n" +
		"- name: b\n  cluster:\n    server: " + serverB + "\n" +
		"contexts:\n" +
		"- name: a\n  context:\n    cluster: a\n    user: a\n" +
		"- name: b\n  context:\n    cluster: b\n    user: b\n" +
		"current-context: a\n" +
		"users:\n" +
		"- name: a\n  user: {}\n" +
		"- name: b\n  user: {}\n"
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	return p
}

func TestRestConfigForKubeconfig_ContextOverride(t *testing.T) {
	dir := t.TempDir()
	file := writeTwoContextKubeconfig(t, dir, "kc.yaml", "https://a.example:6443", "https://b.example:6443")
	t.Setenv("KUBECONFIG", file)

	// No override -> current-context "a".
	cfg, err := restConfigForKubeconfig("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Host != "https://a.example:6443" {
		t.Errorf("without --context expected current-context 'a'; got %q", cfg.Host)
	}

	// --context b selects the other cluster, like `kubectl --context b`.
	cfg, err = restConfigForKubeconfig("", "b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Host != "https://b.example:6443" {
		t.Errorf("--context b should select cluster b; got %q", cfg.Host)
	}

	// A non-existent context is an error, not a silent fallback.
	if _, err := restConfigForKubeconfig("", "does-not-exist"); err == nil {
		t.Errorf("expected error for unknown context")
	}
}
