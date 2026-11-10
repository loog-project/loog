# The .loog file

A `.loog` file is loog's portable, self-contained capture. Every mode reads and
writes the same format, so a file produced by an interactive session, a headless
run, `loog extract`, or `loog fetch` can be browsed the same way with
`loog --replay`.

## What it is

- A **[bbolt](https://github.com/etcd-io/bbolt) database** (a single-file
  embedded key/value store) on disk.
- Holding [revisions](../concepts/revisions.md) keyed by object UID and revision
  ID: full **snapshots** and delta **patches**.
- Records are **MessagePack**-encoded, and by default **s2-compressed**.

You do not edit it by hand; you produce it with loog and read it with loog.

## What a revision stores

Each snapshot or patch carries:

| Field | Meaning |
|-------|---------|
| `ID` | This revision's ID (monotonic per store, shown as short hex). |
| `PreviousID` | The prior revision in this object's chain (`0` for the first). |
| `Object` (snapshot) | The full object state. |
| `Patch` (patch) | The diff versus the previous revision. |
| `Time` | When the change was observed. |
| `Deleted` | Set on the final revision when the object was removed. |

The `PreviousID` links form the per-object chain that reconstruction walks; see
[Revisions, snapshots and patches](../concepts/revisions.md).

## Properties

- **Self-contained.** Replaying needs only the file, no cluster, no network.
- **Read-only replay.** `loog --replay` opens the database read-only and never
  writes, so a capture is safe to archive or share.
- **Compression is transparent.** Files are s2-compressed unless you passed
  `--no-compress`; replay auto-detects and reads either form.
- **Append-safe with intent.** loog refuses to record into an existing file
  unless you pass `--append`, so you never clobber a capture by accident.

## Compression and durability knobs

Set when the file is *written* (see the [CLI reference](cli.md)):

- `--no-compress` — store raw payloads instead of s2-compressed.
- `--snapshot-interval N` — how often a full snapshot is written (affects size
  and reconstruction cost).
- `--no-durable-sync` — skip fsync (faster, unsafe on crash).

## Segments are just many .loog files

The [recorder](../concepts/segments.md) writes a directory of `.loog` files
named `loog-<RFC3339-ish-timestamp>.loog`, one per rotation interval. Each is a
normal, self-contained `.loog` file. `loog extract` and `loog fetch` stitch a
time range across them into a single new `.loog` file.

## Inspecting a file

The intended tools are the TUI (`loog --replay file.loog`) and, from inside it,
the developer console (++colon++) commands `store`, `resources`, `revisions`,
and `inspect` for low-level record dumps.

## See also

- [Revisions, snapshots and patches](../concepts/revisions.md)
- [Segments, rotation and retention](../concepts/segments.md)
- [Replay a capture](../how-to/replay-a-capture.md)
