# Installation

loog is a single static binary. Pick whichever method fits your platform.

## Homebrew (macOS / Linux)

```bash
brew install loog-project/tap/loog
```

## Prebuilt binaries and Linux packages

Grab an archive (`.tar.gz` / `.zip`) or a Linux package (`.deb` / `.rpm` /
`.apk`) from the [latest release](https://github.com/loog-project/loog/releases/latest).
Every release ships checksums, SBOMs, and keyless
[cosign](https://github.com/sigstore/cosign) signatures.

```bash
# Debian / Ubuntu
sudo dpkg -i loog_*_linux_amd64.deb

# Fedora / RHEL
sudo rpm -i loog_*_linux_amd64.rpm
```

## Container image

```bash
docker run --rm -v "$HOME/.kube:/root/.kube:ro" \
  ghcr.io/loog-project/loog:latest -H -o /dev/stdout v1/pods
```

Images are multi-arch (`linux/amd64`, `linux/arm64`). The interactive TUI needs
a TTY (`docker run -it ...`); the image is most useful for headless collection
and for running the [in-cluster recorder](../how-to/deploy-to-kubernetes.md).

## From source

Requires the Go toolchain.

```bash
go install github.com/loog-project/loog@latest
```

Or clone and build:

```bash
git clone https://github.com/loog-project/loog
cd loog
go install .   # or: make build
```

## Verify

```bash
loog version
```

```text
loog v0.x.y
  commit:  1a2b3c4
  built:   2026-09-16T10:00:00Z
  go:      go1.24
  os/arch: darwin/arm64
```

## Next steps

- [Quickstart](quick-start.md) — record and browse a live cluster.
- [Try it without a cluster](without-a-cluster.md) — explore the UI on
  generated data.
- [Install shell completions](../how-to/shell-completions.md) for `bash`, `zsh`,
  `fish`, or `powershell`.
