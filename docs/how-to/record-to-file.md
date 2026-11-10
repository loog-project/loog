# Record to a file

This guide covers the one-shot collection modes: interactive, headless,
temporary vs. persistent files, and safely resuming a recording. For the
always-on in-cluster recorder, see [Run the recorder](run-the-recorder.md)
instead.

## Before you begin

- loog [installed](../getting-started/installation.md).
- A kubeconfig that can `get`/`list`/`watch` the resources you want.
- At least one thing to specify: **a resource to watch**, **`-o/--output`** to
  record into, or **`--replay`** to browse an existing file.

## Interactive (default)

Opens the TUI and records in the background.

```bash
loog apps/v1/deployments v1/pods
```

- Revisions go to a store (a temporary file by default).
- Exit the TUI to stop; the temporary store is removed on exit.

To keep the recording, name an output file:

```bash
loog -o session.loog apps/v1/deployments v1/pods
```

## Headless (collect only)

No UI. Runs until interrupted with ++ctrl+c++. Best for long-running collection
and CI.

```bash
loog -H -o history.loog apps/v1/deployments v1/pods
```

If you omit `-o` in headless mode, loog collects into a temporary file and
**keeps it**, printing the path (the file is the deliverable, so it is not
deleted on exit).

## Temporary vs. persistent files

| You ran | Output file | Kept on exit? |
|---------|-------------|---------------|
| `loog v1/pods` (interactive) | auto temp file | No, deleted |
| `loog -o f.loog v1/pods` | `f.loog` | Yes |
| `loog -H v1/pods` (headless) | auto temp file | Yes, path printed |
| `loog -H -o f.loog v1/pods` | `f.loog` | Yes |

## Resuming and protecting recordings

loog refuses to start if `-o` points at a file that already exists, so it never
silently appends to a capture you meant to replay.

```bash
# Fails: file exists
loog -o history.loog v1/pods
# Error: output file "history.loog" already exists; use --append to resume it
# or --replay to browse it read-only

# Resume recording into it
loog --append -o history.loog v1/pods

# Browse it read-only instead
loog --replay history.loog
```

## Tuning durability and size

These flags trade safety or CPU for throughput and disk. Defaults are safe.

- `--snapshot-interval, -s N` — write a full snapshot every `N` patches
  (default `8`). Lower means faster reconstruction, larger files.
- `--no-durable-sync` — skip `fsync` on each commit. Faster, but you can lose
  the tail of a recording on a crash.
- `--disable-cache` — turn off the in-memory cache layer.
- `--no-compress` — store payloads uncompressed. `--replay` auto-detects
  compressed or not.

See [Revisions, snapshots and patches](../concepts/revisions.md) for what these
actually do.

## Verify

- The file exists and grows as changes occur:
  `ls -lh history.loog`.
- It replays: `loog --replay history.loog` opens the TUI over it with no cluster
  connection.

## Troubleshooting

- **"at least one resource argument or the --output flag must be provided"** —
  you passed neither a GVR nor `-o`. Add one.
- **"invalid resource argument"** — check the GVR spelling; it is
  `group/version/resource`, e.g. `apps/v1/deployments` (plural resource, not
  `Deployment`). Core group is empty: `v1/pods`.
- **Nothing is recorded** — loog records changes; a quiet cluster produces the
  initial `ADDED` revisions and then waits. Confirm your filter is not excluding
  everything, and that your RBAC allows `watch`.

## Next steps

- [Replay a capture](replay-a-capture.md).
- [Filter what gets recorded](filter-resources.md).
- [CLI reference](../reference/cli.md) for every flag.
