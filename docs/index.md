---
hide:
  - navigation
---

![loog banner](assets/banner.png){ .lg-banner }

# loog

**loog** watches one or more Kubernetes resource types and records *every change*
as a revision. You browse that history in a fast terminal UI, or collect it
headless into a portable `.loog` file for later.

Think of it as a DVR for your cluster: rewind a Deployment to see exactly when a
replica count changed, catch a controller stuck in a reconcile loop, or pull the
five minutes around an incident and step through it change by change.

<div class="grid cards" markdown>

-   **Watch and record**

    ---

    Point loog at any GVR (`v1/pods`, `apps/v1/deployments`, a CRD...) and it
    records each `ADDED` / `MODIFIED` / `DELETED` event as a revision.

    [:octicons-arrow-right-24: Quickstart](getting-started/quick-start.md)

-   **Browse the history**

    ---

    A three-panel TUI: explore resources, scrub the timeline, and diff any two
    revisions as highlighted YAML.

    [:octicons-arrow-right-24: The terminal UI](concepts/tui.md)

-   **Record continuously**

    ---

    Run loog in-cluster as an always-on "dashcam" that keeps a rolling,
    fixed-size window of history on a PVC.

    [:octicons-arrow-right-24: Run the recorder](how-to/run-the-recorder.md)

-   **Pull it back out**

    ---

    Clip any time range into a single `.loog` file over HTTP or from disk, then
    replay it locally.

    [:octicons-arrow-right-24: Retrieve a time range](how-to/retrieve-a-time-range.md)

</div>

## Try it in ten seconds

```bash
# no cluster needed - runs the TUI on generated data
loog --simulate
```

Inside the TUI, press ++question++ for help and ++ctrl+k++ for the command
palette.

## Watch a live cluster

```bash
# Watch Deployments and Pods cluster-wide in the TUI
loog apps/v1/deployments v1/pods

# Collect headless into a file (Ctrl+C to stop)
loog -H -o history.loog apps/v1/deployments

# Browse a saved recording, read-only, offline
loog --replay history.loog
```

!!! note "loog uses your kubeconfig and RBAC"
    You only see, list, and watch what your credentials already allow. loog
    never needs more access than `kubectl get`/`watch` on the resources you
    point it at.

## Where to go next

- New to loog? Start with [Installation](getting-started/installation.md) then
  the [Quickstart](getting-started/quick-start.md).
- Want the mental model? Read [How loog works](concepts/index.md).
- Looking for a specific flag or key? Jump to the [CLI reference](reference/cli.md)
  or [Keyboard shortcuts](reference/keybindings.md).
- Running it in a cluster? See
  [Deploy the recorder to Kubernetes](how-to/deploy-to-kubernetes.md).
