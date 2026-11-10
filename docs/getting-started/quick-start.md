# Quickstart

This tutorial takes you from zero to browsing live cluster history in a few
minutes. You need [loog installed](installation.md) and a kubeconfig that can
`get`/`list`/`watch` the resources you pick (Kind or Minikube is plenty).

## 1. Watch your first resource

Resources are named as `Group/Version/Resource` (**GVR**) strings. The core
group has an empty group, so Pods are just `v1/pods`.

```bash
loog v1/pods
```

This opens the TUI, starts watching Pods cluster-wide, and records every change
into a temporary file that is deleted when you exit. You should immediately see
resources appear in the left panel as loog lists the current state, then updates
stream in as objects change.

!!! tip "First run feels quiet?"
    loog records *changes*. On a calm cluster you will see the initial `ADDED`
    revision for each Pod, then nothing until something actually changes. Deploy
    or delete something to watch revisions arrive.

## 2. Move around

The TUI has three views. Switch with the function keys:

- ++f1++ **Explorer** — resources grouped by kind, with a revision list and a
  detail pane.
- ++f2++ **Timeline** — every revision across all resources, newest first.
- ++f3++ **Compare** — diff two revisions side by side.

Core navigation:

- ++j++ / ++k++ move down / up in any list.
- ++enter++ selects or expands.
- ++tab++ moves to the next panel; ++1++ / ++2++ / ++3++ focus a panel directly.
- ++question++ opens the full keyboard reference.

Select a resource, then a revision, and the detail pane shows its YAML. Press
++d++ for a diff against the previous revision, or ++p++ to see only the fields
that changed.

## 3. Watch more than one thing

Pass as many GVRs as you like:

```bash
loog apps/v1/deployments v1/services v1/configmaps
```

To add or remove watched kinds *while the TUI is running*, press ++shift+w++ to
open the Watch Manager.

## 4. Narrow it down with a filter

The `-f/--filter` flag takes a boolean expression. Only matching objects are
recorded.

```bash
# Only the prod and kube-system namespaces
loog -f 'Namespaces("prod","kube-system")' v1/pods
```

See [Filter what gets recorded](../how-to/filter-resources.md) for the full
helper set.

## 5. Save a recording to keep

By default loog records to a temp file that vanishes on exit. To keep it, give
an output file:

```bash
loog -o session.loog apps/v1/deployments
```

Later, browse it offline with no cluster connection:

```bash
loog --replay session.loog
```

!!! important "loog will not clobber an existing recording"
    If `session.loog` already exists, loog refuses to start rather than append
    to a file you meant to replay. Use `--append` to resume it or `--replay` to
    browse it read-only.

## What you learned

- GVR arguments select what to watch.
- The TUI has Explorer / Timeline / Compare views.
- Filters scope what gets recorded.
- `-o` keeps a recording; `--replay` browses it later.

## Next steps

- [Record to a file](../how-to/record-to-file.md) — interactive vs headless,
  appending, temp files.
- [Filter what gets recorded](../how-to/filter-resources.md).
- [The terminal UI](../concepts/tui.md) — every panel and mode explained.
