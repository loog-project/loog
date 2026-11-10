# Why loog

## The problem

Kubernetes objects change constantly, and most of that change is invisible after
the fact. `kubectl get` shows you the object *now*. `kubectl describe` shows a
handful of recent events that age out fast. If a Deployment's image changed at
2am, a HorizontalPodAutoscaler flapped, or a controller quietly rewrote an
annotation in a loop, you usually find out long after the evidence is gone.

The usual answers each fall short:

- **Events** are short-lived (an hour by default) and only capture a curated set
  of transitions, not the full object.
- **Audit logs** record API requests, not reconstructable object state, and are
  painful to read for "what did this one resource look like over time".
- **GitOps history** tells you what you *declared*, not what the cluster and its
  controllers actually did to the live object.

## What loog does instead

loog watches resources the same way a controller does (`list` + `watch`) and
stores every observed state as a **revision**. Because it keeps the full object
per change, you can:

- reconstruct exactly what a resource looked like at any recorded moment,
- diff any two revisions as highlighted YAML,
- see *when* and *how often* something changed, and
- spot patterns like reconcile loops and high-churn ("hot") resources.

It does this with no controller to install for the interactive case: run it from
your laptop against your kubeconfig and it starts recording immediately.

## Two ways to run it

| Mode | What it is | When to use it |
|------|------------|----------------|
| **Interactive / headless** | A one-shot session from your machine that watches and records into a single `.loog` file. | Debugging something happening *now*, or collecting a capture during a test. |
| **The recorder ("dashcam")** | An in-cluster Deployment that records continuously into rotating segments with a retention window. | Answering "what happened at 2am last Tuesday?" after the fact. |

Both write the same `.loog` format, so a capture from either can be browsed with
the same `loog --replay`.

## Design goals

- **Cheap to keep running.** Changes are stored as compact patches with periodic
  full snapshots, compressed on disk, so long recordings stay small.
- **Self-contained captures.** A `.loog` file (or an extracted time range) replays
  offline with no cluster connection and never mutates the source.
- **Least privilege.** loog only ever needs read access (`get`/`list`/`watch`) to
  the resources you record.
- **Fast to navigate.** The TUI is built for scrubbing thousands of revisions,
  not just reading a single object.

## See also

- [How loog works](../concepts/index.md) for the architecture.
- [Revisions, snapshots and patches](../concepts/revisions.md) for the storage model.
- [Quickstart](../getting-started/quick-start.md) to try it.
