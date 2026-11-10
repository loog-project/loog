# CLI reference

loog is one binary with a default command (watch/browse) and a few subcommands.
Run `loog --help` or `loog <command> --help` for the built-in version of this
page.

```text
loog [FLAGS] [RESOURCES...]        watch + browse/collect (default)
loog record  [FLAGS] RESOURCES...  continuous in-cluster recorder
loog extract [FLAGS]               clip a time range from segment files
loog fetch   [FLAGS]               clip a time range from a recorder over HTTP
loog completion <shell>            print a shell completion script
loog version                       print version / build info
```

## Resource arguments (GVR)

Resources are `Group/Version/Resource` strings. The core group is empty:

| Example | Group | Version | Resource |
|---------|-------|---------|----------|
| `v1/pods` | *(core)* | v1 | pods |
| `apps/v1/deployments` | apps | v1 | deployments |
| `batch/v1/jobs` | batch | v1 | jobs |
| `example.com/v1alpha1/widgets` | example.com | v1alpha1 | widgets |

Use the plural resource name, not the kind (`deployments`, not `Deployment`).

## Persistent flags

Available on every command.

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `$HOME/.loog.yaml` | Config file path. |
| `--kubeconfig` | `$KUBECONFIG` or `$HOME/.kube/config` | Kubeconfig path (overrides `$KUBECONFIG`). |
| `--context` | current context | Kubeconfig context to use. |
| `--debug` | `false` | Write verbose logs to `debug.log`. |
| `--truncate-debug` | `false` | Truncate `debug.log` on startup. |

## `loog` (default command)

Watch resources and browse them in the TUI, or collect headless. Requires at
least one of: a resource argument, `--output`, or `--replay`.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--output` | `-o` | temp file | Path to the `.loog` output file. |
| `--filter` | `-f` | `All()` | Filter expression ([reference](filters.md)). |
| `--headless` | `-H` | `false` | No TUI; collect only (Ctrl+C to stop). |
| `--append` | | `false` | Allow `--output` to resume an existing file. |
| `--replay` | | | Open an existing `.loog` file read-only. |
| `--simulate` | | `false` | Run the TUI on generated data (no cluster). |
| `--snapshot-interval` | `-s` | `8` | Full snapshot every N patches. |
| `--no-durable-sync` | | `false` | Skip fsync on each commit (faster, unsafe on crash). |
| `--disable-cache` | | `false` | Disable the in-memory cache layer. |
| `--no-compress` | | `false` | Store payloads uncompressed. |

Examples:

```bash
loog v1/pods
loog apps/v1/deployments v1/services
loog -H -o history.loog v1/pods
loog --replay history.loog
loog --simulate
```

!!! note "Mutually exclusive"
    `--replay` cannot be combined with resource args, `--output`, `--append`,
    `--headless`, or `--simulate`.

## `loog record`

Continuous recorder that writes rotating segments. Requires at least one
resource. See [Run the recorder](../how-to/run-the-recorder.md).

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--dir` | | `/var/lib/loog` | Directory for segment files. |
| `--interval` | | `1h` | How often to roll to a new segment. |
| `--retention` | | `720h` (30d) | Delete segments older than this (`0` = keep all). |
| `--max-segments` | | `0` | Hard cap on retained segments (oldest deleted first; `0` = unlimited). |
| `--filter` | `-f` | `All()` | Filter expression selecting what to record. |
| `--snapshot-interval` | `-s` | `8` | Full snapshot every N patches. |
| `--no-durable-sync` | | `false` | Skip periodic fsync. |
| `--no-compress` | | `false` | Disable s2 compression. |
| `--http` | | *(off)* | Serve the retrieval API on this address, e.g. `:8080`. |
| `--disable-http2` | | `true` | Use HTTP/1.1, one connection per watch (see [robustness](configuration.md#recorder-robustness)). |
| `--qps` | | `20` | Client-side QPS limit toward the apiserver. |
| `--burst` | | `30` | Client-side burst limit toward the apiserver. |

```bash
loog record --dir /data apps/v1/deployments v1/configmaps
loog record --dir /data --http :8080 --interval 10m --retention 720h v1/pods
```

## `loog extract`

Clip a time range from segment files on disk into one `.loog` file. See
[Retrieve a time range](../how-to/retrieve-a-time-range.md).

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--dir` | | *(required)* | Directory containing segment files. |
| `--from` | | | Window start (RFC3339 or `YYYY-MM-DD HH:MM:SS`); empty = beginning. |
| `--to` | | | Window end; empty = now. |
| `--since` | | | Relative window ending now, e.g. `2h`, `30m` (overrides `--from`/`--to`). |
| `--filter` | `-f` | `All()` | Filter expression. |
| `--output` | `-o` | temp file | Output `.loog` file. |
| `--replay` | | `false` | Open the result in the TUI after extraction. |
| `--compress` | | `true` | Compress the output with s2. |

```bash
loog extract --dir /data --since 2h -o out.loog --replay
```

## `loog fetch`

Clip a time range from a running recorder over HTTP. See
[Retrieve a time range](../how-to/retrieve-a-time-range.md).

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--server` | | *(required)* | Recorder API address, e.g. `localhost:8080` (scheme optional). |
| `--from` | | | Window start; empty = beginning. |
| `--to` | | | Window end; empty = now. |
| `--since` | | | Relative window ending now (overrides `--from`/`--to`). |
| `--filter` | `-f` | recorder's filter | Filter expression; empty uses the recorder's own filter. |
| `--output` | `-o` | temp file | Output `.loog` file. |
| `--replay` | | `false` | Open the result in the TUI after download. |

```bash
loog fetch --server localhost:8080 --since 1h --replay
```

## `loog completion`

Print a completion script for `bash`, `zsh`, `fish`, or `powershell`. See
[Install shell completions](../how-to/shell-completions.md).

```bash
loog completion zsh
```

## `loog version`

Print version, commit, build date, Go version, and OS/arch.

```bash
loog version
```

## Exit status

loog exits non-zero on error (bad flags, invalid GVR, kubeconfig or store
failure). A clean TUI quit or a Ctrl+C stop of a headless/record session exits
zero.

## See also

- [Configuration and environment](configuration.md)
- [Filter expressions](filters.md)
- [Keyboard shortcuts](keybindings.md)
