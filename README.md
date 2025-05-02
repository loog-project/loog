![banner](./assets/banner.png)

_LOOG_ is a small program that records every change made to one or more Kubernetes resources and lets you browse those revisions.

https://github.com/user-attachments/assets/7013fe8b-fbe3-42a9-96c7-9a2fad8fabf5


---

## Install (using Go)

```bash
go install github.com/loog-project/loog/cmd/loog@latest
```

---

## Usage

```text
$ loog [flags]

Usage of loog:
  -filter-expr string
        expr filter (default "All()")
  -kubeconfig string
         (default "/Users/I550629/.kube/config")
  -no-cache
        if set to true, the store won't cache the data
  -non-interactive
        set to true to disable the UI
  -not-durable
        if set to true, the store won't fsync every commit
  -out string
        dump output file
  -resource value
        <group>/<version>/<resource> (repeatable)
  -snapshot-every uint
        patches until snapshot (default 8)
```

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
