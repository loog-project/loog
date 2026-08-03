![banner](./assets/banner.png)

<blockquote align="center"><i>LOOG</i> is a small TUI program that records every change made to one or more Kubernetes resources and lets you browse those revisions.</blockquote>

<p align="center">
[
  <a href="#quick-start">Quick Start</a> •
  <a href="#installation">Installation</a> •
  <a href="#contributing">Contributing</a> •
]
</p>

https://github.com/user-attachments/assets/29990ea7-21ba-4003-bddf-543205af44f9

---

## Usage

`loog` watches Kubernetes resources and records every change as a revision.
You can run it interactively (TUI) to explore history as it happens,
or headless to collect revisions into a `.loog` file for later analysis (or both).

> [!TIP]
> Run `loog --help` for the full flag reference and shell completions.

### Quick start

```bash
# Watch Pods cluster-wide in the TUI (temporary file; deleted on exit)
loog v1/pods
```

```bash
# Watch Deployments and Services
loog apps/v1/deployments v1/services
```

```bash
# Limit what gets recorded using a filter (see "Filtering" below)
loog -f 'Namespaces("prod","kube-system")' v1/pods
```

**Resources** are specified as Group/Version/Resource (`GVR`) strings, e.g. `v1/pods`, `apps/v1/deployments`,
`batch/v1/jobs`. You must provide **at least one resource** to watch, an `--output [FILE]` to record into, or
`--replay [FILE]` to browse an existing recording.

> [!TIP]
> Just want to try the UI without a cluster? `loog --simulate` runs the TUI on generated data.
> Inside the TUI, press `?` for help and `ctrl+k` for the command palette.

> [!NOTE]
> _LOOG_ uses your kubeconfig and RBAC. You'll only see/list/watch what your credentials allow.

### Interactive vs. headless

```bash
# Interactive (TUI) - default mode
loog apps/v1/deployments v1/pods
```

* Opens a terminal UI and starts watching the given resources cluster-wide.
* Revisions are written to a store (temp file by default).
* Exit the TUI to stop; the temp store is removed on exit.

```bash
# Headless - collect only, no TUI (Ctrl+C to stop)
loog -H -o history.loog apps/v1/deployments v1/pods
```

* Runs without UI and records revisions to `history.loog` until interrupted.
* Safer for long-running collection jobs and CI.

```bash
# Browse an existing recording read-only (no cluster connection)
loog --replay history.loog
```

* Opens the TUI over the saved revisions in `history.loog` **without** connecting to Kubernetes or modifying the file.

### Recording to a file

Use `-o/--output FILE` to record revisions to a specific file.

> [!IMPORTANT]
> If `FILE` already exists, `loog` **refuses to start** so it never appends to a recording you meant to replay.
> Pass `--append` to resume recording into it, or `--replay` to browse it read-only.

```bash
# Start a new recording (fails if the file already exists)
loog -o history.loog v1/pods

# Resume recording into an existing file
loog --append -o history.loog v1/pods
```

### Filtering

The `-f/--filter` flag takes an [expr-lang](https://github.com/expr-lang/expr) boolean expression.
By default, it's `All()` (record everything).

**Built-in helpers** make it easy to filter by namespace, name, or both:

* `Namespaces("ns1", "ns2", ...)` (alias: `Namespace(...)`) checks if the object is in one of the given namespaces.
* `Names("n1", "n2", ...)` (alias: `Name(...)`) checks if the object has one of the given names.
* `Namespaced("namespace", "name")` checks if the object is a namespaced resource with the given namespace and name.
* `LabelExists("key1", "key2", ...)` checks if the object has any of the given labels.
* `Label("key", "value")` checks if the object has a label with the given key and value.

**Examples:**

```bash
# Only objects in prod and kube-system
loog -f 'Namespaces("prod","kube-system")' v1/pods

# Only specific objects by name (cluster-wide)
loog -f 'Names("nginx","api")' v1/services

# A single namespaced object
loog -f 'Namespaced("default","nginx-deployment")' apps/v1/deployments

# Combine with boolean logic
loog -f 'Namespaces("prod") && !Names("tmp","scratch")' v1/configmaps
```

You can also reference the live event and object:

* `Event.Type` is one of `ADDED|MODIFIED|DELETED`
* `Object` is a Kubernetes `unstructured.Unstructured`

```bash
# Only record live MODIFIED events in prod
loog -f 'Event.Type == "MODIFIED" && Namespace("prod")' v1/pods

# Filter by label
loog -f 'Object.GetLabels()["app"] == "web" && Object.GetNamespace() == "adm"' apps/v1/deployments
```

> [!IMPORTANT]
> Filters are evaluated **before writing** live events.
> **Non-matching resources are *not added* to the database.**
> When replaying an existing `.loog` file (`--replay`), the filter acts as a **view** and never modifies the file.

### Performance & Durability

- `--snapshot-interval, -s <N>`: write a full snapshot every N patches (default `8`).
- `--no-durable-sync`: skip fsync on each commit (higher throughput, **unsafe on crashes**).
- `--disable-cache`: disable the in-memory cache layer.
- `--no-compress`: store payloads uncompressed (larger file, slightly less CPU). Files are compressed by default;
  `--replay` detects and reads either automatically.

### Kubeconfig & Debug

- `--kubeconfig <path>` (default: `$HOME/.kube/config`)
- `--debug`: write verbose logs to `debug.log` (`--truncate-debug` to start fresh)
  - this is useful for debugging the TUI

---

## Installation

### `loog` base binary

```bash
go install github.com/loog-project/loog@latest
```

or clone and build from source:

```bash
git clone https://github.com/loog-project/loog
cd loog
go install . # or: make build
```

#### Shell Completions

_LOOG_ supports shell completions for `bash`, `zsh`, `fish` and `powershell`.
To install completions for `bash`, `zsh` and `fish`, add the following lines to your shell configuration file

```bash
# bash
source <(loog completion bash)

# zsh
source <(loog completion zsh)

# fish
loog completion fish | source

# powershell
loog completion powershell | Out-String | Invoke-Expression
```

### `kubectl` plugin

> You can use `make link-kubectl` to automatically install _LOOG_ as a `kubectl` plugin.
> Use `make unlink-kubectl` to remove the plugin.

To install _LOOG_ as a `kubectl` plugin, copy or link the _LOOG_ binary to your `PATH`:

```bash
ln -s $(which loog) $(dirname $(which loog))/kubectl-observe
kubectl observe v1/configmaps
```

Note that the plugin does not support shell completions yet.

### `k9s` plugin

> You can use `make link-k9s` to automatically install _LOOG_-shortcuts for `k9s`.
> Use `make unlink-k9s` to remove the shortcuts.

To install _LOOG_-shortcuts for `k9s`, copy the `compat/k9s/plugins.yaml` to your
[`k9s` plugins directory](https://github.com/derailed/k9s#k9s-configuration) or extend your existing `plugins.yaml`.

```bash
# macOS
mkdir -p ~/Library/Application\ Support/k9s/plugins
cp ./compat/k9s/plugins.yaml ~/Library/Application\ Support/k9s/plugins/loog-plugins.yaml

# Unix
mkdir -p ~/.config/k9s/plugins
cp ./compat/k9s/plugins.yaml ~/.config/k9s/plugins/loog-plugins.yaml
```

---

## Contributing

The code base is **very young and still moving quickly**. Pull requests are welcome, but opening an issue first avoids
wasted work if the surrounding code changes while you are developing.

Development requires the usual Go tool-chain and a running Kubernetes cluster (Kind or Minikube is enough).
Unit tests run with `go test ./...`.
