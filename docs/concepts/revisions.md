# Revisions, snapshots and patches

Every change loog records is a **revision**. Understanding how revisions are
stored explains loog's flags for durability and size, and why long recordings
stay small.

## Revisions

A revision is one observed state of one object, identified by the object's UID
plus a monotonic **revision ID** (rendered as a short hex string like `002a`).
Each revision knows its `PreviousID`, forming a per-object chain:

```
object UID ──▶ rev 1 ──▶ rev 2 ──▶ rev 3 ──▶ ...
             (snapshot) (patch)   (patch)
```

A revision also carries a timestamp and a `Deleted` flag (see
[Delete events](#delete-events)).

## Snapshots vs. patches

loog stores a revision in one of two shapes:

- A **snapshot** holds the *full* object state. It stands alone, no previous
  revision is needed to read it.
- A **patch** holds only the *diff* against the previous revision. It is small,
  but reconstructing the object requires walking back to the last snapshot and
  applying every patch in between.

The first time loog sees an object it writes a snapshot. After that it writes
patches, with a fresh full snapshot every `--snapshot-interval` patches
(default `8`). This bounds how many patches must be replayed to rebuild any
state.

```
rev:   1        2      3      4  ...  9        10
form:  snap     patch  patch  patch   snap     patch
       └── reconstruct rev 8 = snap#1 + patches 2..8 ──┘
```

### Choosing the snapshot interval

`--snapshot-interval N` (alias `-s`) trades file size against reconstruction
cost:

- **Smaller `N`** → more snapshots → larger files, but faster to reconstruct any
  point and more resilient (a corrupt patch affects fewer revisions).
- **Larger `N`** → fewer snapshots → smaller files, but more patches to replay.

The default of 8 is a good balance for typical objects.

## The diff format

Patches use loog's own map-diff format
([`pkg/diffmap`](https://github.com/loog-project/loog/tree/main/pkg/diffmap)),
which records added, modified, and removed keys within the nested object. It is
tuned for Kubernetes objects (deeply nested maps) and drives both storage and
the on-screen diff view.

loog strips `metadata.managedFields` before committing, it is large,
churn-heavy, and almost never what you are looking at.

## Delete events

When an object is deleted, loog records a final revision with its `Deleted` flag
set; the stored object is the last observed state before removal. In the TUI,
deleted resources and their final revision are shown struck through, so you can
tell "gone" from "idle". Delete revisions replay like any other, so an extracted
window faithfully shows objects that disappeared during it.

## Durability and compression

Two flags control how safely and compactly revisions hit disk:

- **`--no-durable-sync`** — by default loog fsyncs so a crash cannot lose
  acknowledged writes (batched on a short interval). This flag skips fsync for
  higher throughput at the cost of possibly losing the tail on a crash.
- **`--no-compress`** — payloads are s2-compressed by default. This stores them
  raw (slightly less CPU, larger files). Replay auto-detects either form.

There is also **`--disable-cache`**, which turns off the in-memory cache layer
the tracker uses to avoid re-reading recent state; leave it on unless you are
debugging.

## See also

- [The .loog file](../reference/loog-file.md) — the on-disk container.
- [Segments, rotation and retention](segments.md) — how the recorder chains many
  files.
- [Change analysis](analysis.md) — what loog derives from a revision chain.
