# loog

> A small program that records every change made to one or more Kubernetes resources and lets you browse those revisions.

![demo](assets/demo.mp4)

---

## Install (using Go)

```bash
go install github.com/loog-project/loog/cmd/loog@latest
```

---

## Usage

```text
loog [flags]
```

| Flag               | Description                                                                                                           | Default                 |
|--------------------|-----------------------------------------------------------------------------------------------------------------------|-------------------------|
| `-kubeconfig`      | Path to the kube‑config file used for in‑cluster or remote access                                                     | `$HOME/.kube/config`    |
| `-resource`        | Fully qualified resource to watch (`<group>/<version>/<resource>`). Repeat for multiple resources.                    | none (add at least one) |
| `-out`             | Path of the bolt‑DB file that stores snapshots and patches. If empty, a temporary file is created and deleted on exit | ‘’                      |
| `-not-durable`     | Disable `fsync` after every write (faster, but crashes may corrupt the DB)                                            | `false`                 |
| `-no-cache`        | Skip the in‑memory hot cache                                                                                          | `false`                 |
| `-snapshot-every`  | Store a full snapshot after *N* patches                                                                               | `8`                     |
| `-filter-expr`     | \[expr‑lang] filter executed for every incoming event                                                                 | `All()`                 |
| `-non-interactive` | Do not start the TUI; collect and persist only                                                                        | `false`                 |

Example: watch Deployments and ConfigMaps, keep data in `state.loog`, show the UI.

```bash
loog \
  -resource apps/v1/deployments \
  -resource v1/configmaps \
  -out state.loog
```

Quit with `q` or `Ctrl‑C`.

---

## Contributing

The code base is **very young and still moving quickly**. Pull requests are welcome, but opening an issue first avoids
wasted work if the surrounding code changes while you are developing.

Development requires the usual Go tool‑chain and a running Kubernetes cluster (Kind or Minikube is enough). 
Unit tests run with `go test ./...`.
