# Replay a capture

Replay opens an existing `.loog` file in the TUI, read-only. It never connects
to a cluster and never modifies the file, so it is safe to hand a capture to
someone else (or to your future self) for offline analysis.

## Before you begin

- A `.loog` file, from any source:
    - `loog -o file.loog ...` (interactive or headless collection),
    - `loog extract ...` (clipped from recorder segments on disk),
    - `loog fetch ...` (pulled from an in-cluster recorder over HTTP).

## Open it

```bash
loog --replay history.loog
```

The TUI loads the full history up front, sorts the timeline chronologically, and
then behaves exactly like a live session, minus any incoming updates.

## Replay with a view filter

In replay mode, `-f/--filter` acts as a **view**: it hides non-matching objects
but never rewrites the file.

```bash
# Browse only the payments namespace within a capture
loog --replay history.loog -f 'Namespaces("payments")'
```

This is different from recording, where the filter decides what gets *written*.
See [The filtering model](../concepts/filtering.md).

## What you can and cannot do

- **Can:** browse all views, diff revisions, export a revision's YAML (++e++),
  copy YAML to the clipboard (++y++), inspect raw records (++r++).
- **Cannot:** add or remove watches, resume recording into the file, or change
  its contents.

!!! important "Replay cannot be combined with collection flags"
    `--replay` refuses to run alongside resource arguments, `-o/--output`,
    `--append`, `-H/--headless`, or `--simulate`. It is a pure browse session.

## Verify

The file opens and shows revisions. If it is empty, the capture recorded no
changes in the window it covers (common for very short or heavily filtered
captures).

## Troubleshooting

- **"--replay file ... does not exist or is not a file"** — check the path.
- **Opens but looks empty** — the capture may be filtered down to nothing by
  `-f`, or it genuinely contains no revisions. Drop the filter and retry.
- **Older/compressed files** — replay auto-detects s2-compressed and
  uncompressed payloads, so both open without extra flags.

## Next steps

- [The terminal UI](../concepts/tui.md) — get the most out of a capture.
- [Retrieve a time range](retrieve-a-time-range.md) — produce captures from a
  running recorder.
