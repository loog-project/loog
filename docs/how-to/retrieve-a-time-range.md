# Retrieve a time range

Both retrieval commands clip a time window out of recorder segments into a
single, portable `.loog` file, preserving original timestamps and delete
markers, then optionally open it in the TUI. Pick based on how you reach the
data:

| Command | Reaches | Use when |
|---------|---------|----------|
| `loog fetch` | a running recorder over **HTTP** | The recorder is up and you can port-forward to it. |
| `loog extract` | segment files on **disk** | You have the files directly (mounted PVC, `kubectl cp`, rsync). |

## Before you begin

- A recorder that has been running (see [Run the recorder](run-the-recorder.md)).
- For `fetch`: network access to its `--http` address (a port-forward is
  simplest).
- For `extract`: filesystem access to the segment directory.

## Selecting the window

All three selectors are shared by both commands (and the HTTP API):

- `--since 2h` / `--since 30m` — relative window ending *now*. Takes precedence
  over `--from`/`--to`.
- `--from` / `--to` — absolute bounds. Accept RFC3339
  (`2026-09-10T14:00:00Z`) or shorthands like `2026-09-10 14:00:00` and
  `2026-09-10` (interpreted in the machine's local timezone). An empty `--from`
  means beginning-of-time; an empty `--to` means now.

## Fetch over HTTP

Port-forward the recorder, then fetch:

```bash
kubectl -n loog port-forward svc/loog-recorder 8080:8080

# Last 30 minutes, browse immediately
loog fetch --server localhost:8080 --since 30m --replay

# A precise incident window, one namespace, saved to a file
loog fetch --server localhost:8080 \
  --from 2026-09-10T14:00:00Z --to 2026-09-10T15:00:00Z \
  -f 'Namespaces("payments")' -o incident.loog
loog --replay incident.loog
```

When `-f/--filter` is omitted, `fetch` uses the recorder's own filter. The
`--server` value may omit the scheme (`localhost:8080` becomes
`http://localhost:8080`).

## Extract from files

Point `--dir` at a directory of segment files you can read:

```bash
# Everything from the last 2 hours, then open it
loog extract --dir /path/to/segments --since 2h -o out.loog --replay

# A precise window, only one namespace
loog extract --dir /path/to/segments \
  --from 2026-09-10T14:00:00Z --to 2026-09-10T15:00:00Z \
  -f 'Namespaces("payments")' -o incident.loog
```

If you omit `-o`, the tool writes to a temporary file and prints its path.

## Verify

Both commands print a summary:

```text
Wrote 214 revisions across 31 resources to out.loog
```

`fetch` also surfaces `X-Loog-Revisions` / `X-Loog-Resources` from the server.
Open the result with `loog --replay out.loog` (or pass `--replay` to do it in
one step).

## Troubleshooting

- **Empty or tiny result** — the window may not overlap any recorded change, or
  your filter excluded everything. Widen `--since`, or list coverage with
  `curl -s localhost:8080/segments | jq`.
- **`fetch` returns fresher data than `extract`** — expected. `fetch` reads the
  active segment through the recorder's own handle; `extract` on an offline copy
  cannot open the locked active file and only sees rolled segments.
- **`--to is before --from`** — check the bounds; `to` must not precede `from`.
- **`invalid --since`** — use a Go duration like `2h`, `30m`, `90s`.

## Next steps

- [Recorder HTTP API reference](../reference/http-api.md) — endpoints and query
  params.
- [Segments, rotation and retention](../concepts/segments.md) — why windows map
  cleanly onto files.
