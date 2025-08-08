![banner](./assets/banner.png)

_LOOG_ is a small program that records every change made to one or more Kubernetes resources and lets you browse those
revisions.

https://github.com/user-attachments/assets/7013fe8b-fbe3-42a9-96c7-9a2fad8fabf5


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

> [!NOTE]
> **Resources** are specified as Group/Version/Resource (`GVR`) strings, e.g. `v1/pods`, `apps/v1/deployments`,
`batch/v1/jobs`.
> You must provide **at least one resource** to watch or load an older recording using `--output [FILE]` file (or both).

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

* Runs without UI and appends revisions to `history.loog` until interrupted.
* Safer for long-running collection jobs and CI.

```bash
# Explore an existing file later (no new watches started)
loog -o history.loog
```

* Opens the TUI over the saved revisions in `history.loog`.

### Filtering

The `-f/--filter` flag takes an [expr-lang](https://github.com/expr-lang/expr) boolean expression.
By default, it's `All()` (record everything).

**Built-in helpers** make it easy to filter by namespace, name, or both:

* `Namespaces("ns1", "ns2", ...)` (alias: `Namespace(...)`) checks if the object is in one of the given namespaces.
* `Names("n1", "n2", ...)` (alias: `Name(...)`) checks if the object has one of the given names.
* `Namespaced("namespace", "name")` checks if the object is a namespaced resource with the given namespace and name.
* `HasLabels("key1", "key2", ...)` (alias: `HasLabel(...)`) checks if the object has any of the given labels.
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
> When opening an existing `.loog` file, the filter acts as a **view** in the UI (it doesn't delete data).

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
go install .
```

#### Shell Completions

_LOOG_ supports shell completions for `bash`, `zsh`, `fish` and `powershell`.
To install completions for `bash`, `zsh` and `fish`, add the following lines to your shell configuration file

```bash
source <(loog completion bash)  # for bash
source <(loog completion zsh)   # for zsh
source <(loog completion fish)  # for fish
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

## Contributing

The code base is **very young and still moving quickly**. Pull requests are welcome, but opening an issue first avoids
wasted work if the surrounding code changes while you are developing.

Development requires the usual Go tool-chain and a running Kubernetes cluster (Kind or Minikube is enough).
Unit tests run with `go test ./...`.
