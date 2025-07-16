![banner](./assets/banner.png)

_LOOG_ is a small program that records every change made to one or more Kubernetes resources and lets you browse those
revisions.

https://github.com/user-attachments/assets/7013fe8b-fbe3-42a9-96c7-9a2fad8fabf5


---

## Installation

### `loog` base binary

```bash
go install github.com/loog-project/loog/cmd/loog@latest
```

or clone and build from source:

```bash
git clone https://github.com/loog-project/loog
cd loog
go install ./cmd/loog
```

### `kubectl` plugin

To install `loog` as a `kubectl` plugin, copy the binary to your `PATH`:

```bash
ln -s $(which loog) $(dirname $(which loog))/kubectl-loog
```

### Completions

Shell completions for `zsh` can be found in `compat/zsh/_loog`. To install, copy it to your `fpath`:

```bash
mkdir -p ~/.zsh/completions
ln -s $(pwd)/compat/zsh/_loog ~/.zsh/completions/_loog
echo 'fpath=(~/.zsh/completions $fpath)' >> ~/.zshrc
source ~/.zshrc
```

---

## Usage

```text
$ loog [flags] [<resource> ...]

Usage of loog:
  -filter-expr [string]     expr filter (default "All()")
  -kubeconfig [string]      path to the kubeconfig file
        (default "~/.kube/config")
  -no-cache                 if set to true, the store won't cache the data
  -non-interactive          if set to true, the UI won't be shown
  -not-durable              if set to true, the store won't fsync every commit
  -out string               output file of the revisions
  -snapshot-every [uint]    patches until snapshot (default 8)
```

Example: watch Deployments and ConfigMaps, keep data in `state.loog`, show the UI.

```bash
loog -out state.loog apps/v1/deployments v1/configmaps
```

---

## Contributing

The code base is **very young and still moving quickly**. Pull requests are welcome, but opening an issue first avoids
wasted work if the surrounding code changes while you are developing.

Development requires the usual Go tool‑chain and a running Kubernetes cluster (Kind or Minikube is enough).
Unit tests run with `go test ./...`.
