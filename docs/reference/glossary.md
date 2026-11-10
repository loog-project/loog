# Glossary

Short definitions of the terms used throughout these docs.

**Active segment**
: The segment file the [recorder](../concepts/segments.md) is currently writing.
  bbolt locks it exclusively, so `loog extract` on an offline copy cannot read
  it; `loog fetch` can, through the recorder's own handle.

**Capture**
: A `.loog` file holding recorded revisions. Produced by any mode; browsed with
  `loog --replay`.

**Change tag**
: A label loog derives for a revision describing what changed (`image`,
  `replicas`, `spec`, `status`, `labels`, `config`, `unknown`). See
  [Change analysis](../concepts/analysis.md).

**GVR**
: Group / Version / Resource, the string that names a resource type, e.g.
  `apps/v1/deployments`. The core group is empty (`v1/pods`).

**Headless mode**
: `loog -H`, collect revisions to a file with no TUI.

**Hot resource**
: A resource changing frequently, flagged `▲` (hot) or `△` (warm) in the UI.

**Patch**
: A revision stored as the diff against the previous revision, rather than a full
  object. See [Revisions, snapshots and patches](../concepts/revisions.md).

**Reconcile loop**
: A resource oscillating between repeating states, detected by loog and marked
  `↻`. See [Change analysis](../concepts/analysis.md).

**Recorder ("dashcam")**
: `loog record`, the always-on, in-cluster collector that writes rotating
  segments with a retention window.

**Replay**
: `loog --replay`, opening an existing capture read-only in the TUI, with no
  cluster connection.

**Retention**
: How long the recorder keeps rolled segments before pruning them
  (`--retention`); optionally combined with a count cap (`--max-segments`).

**Revision**
: One observed state of one object, identified by object UID plus a monotonic
  revision ID. Stored as a snapshot or a patch.

**Rotation**
: The recorder closing the active segment and opening a new one every
  `--interval`, resetting the tracker cache so each segment is self-contained.

**Segment**
: One `.loog` file in a recorder directory, covering a slice of time. See
  [Segments, rotation and retention](../concepts/segments.md).

**Snapshot**
: A revision storing the full object state, standing alone without needing prior
  revisions.

**Snapshot interval**
: `--snapshot-interval N`, write a full snapshot every N patches to bound
  reconstruction cost.

**Tracker (tracker service)**
: The component that decides snapshot-vs-patch, assigns revision IDs, and commits
  to the store.

**Watch mux**
: The internal component that runs one informer per GVR and merges their events
  into a single stream. See [How loog works](../concepts/index.md).
