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

#### `zsh` shell completions

Shell completions for `zsh` can be found in `compat/zsh/_loog`. To install, copy it to your `fpath`:

```bash
echo 'fpath=("'$(pwd)'/compat/zsh" $fpath)' >> ~/.zshrc
echo 'autoload -Uz compinit && compinit' >> ~/.zshrc
source ~/.zshrc
```

### `kubectl` plugin

To install _LOOG_ as a `kubectl` plugin, copy or link the _LOOG_ binary to your `PATH`:

```bash
ln -s $(which loog) $(dirname $(which loog))/kubectl-observe
kubectl observe v1/configmaps
```

Note that the plugin does not support shell completions yet.

### `k9s` plugin

To install _LOOG_-shortcuts for `k9s`, copy the `compat/k9s/plugins.yaml` to your
[`k9s` config directory](https://github.com/derailed/k9s#k9s-configuration) or extend your existing `plugins.yaml`.

```bash
# macOS
cp compat/k9s/plugins.yaml ~/Library/Application\ Support/k9s/plugins.yaml
# Unix
cp compat/k9s/plugins.yaml ~/.config/k9s/plugins.yaml
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
  -out [string]             output file of the revisions
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
