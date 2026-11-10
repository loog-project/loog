# Configuration and environment

Most of loog is driven by flags (see the [CLI reference](cli.md)). A few things
can also come from a config file or the environment, and the recorder has a set
of connection-robustness knobs worth understanding.

## Config file

On startup loog reads a config file if present:

- Default: `$HOME/.loog.yaml`.
- Override with `--config /path/to/file.yaml`.

Keys mirror flag names. Flags bound to the config/env include:

- `kubeconfig`
- `debug`
- `truncate-debug`
- `no-durable-sync`
- `disable-cache`
- `snapshot-interval`

```yaml title="~/.loog.yaml"
kubeconfig: /home/me/.kube/config
snapshot-interval: 16
debug: false
```

Precedence is the usual: command-line flags override config-file values, which
override defaults.

## Environment variables

loog reads environment variables automatically for the bound keys above
(uppercased). It also honors standard Kubernetes and HTTP/2 variables:

| Variable | Effect |
|----------|--------|
| `KUBECONFIG` | Kubeconfig path(s), same as `kubectl`. A merged list is supported. `--kubeconfig` overrides it. |
| `HTTP2_READ_IDLE_TIMEOUT_SECONDS` | HTTP/2 keepalive ping interval (only relevant when HTTP/2 is enabled). |
| `HTTP2_PING_TIMEOUT_SECONDS` | HTTP/2 keepalive ping timeout. |

## Kubeconfig resolution

When `--kubeconfig` is unset, loog resolves the kubeconfig the way `kubectl`
does:

1. `$KUBECONFIG` if set (including a merged, colon-separated list of files),
2. otherwise `$HOME/.kube/config`.

Select a context with `--context <name>` (defaults to the current context). In
cluster, the recorder uses the pod's ServiceAccount automatically, no kubeconfig
needed.

## Debug logging

- `--debug` writes verbose logs to `debug.log` in the working directory. This is
  the way to debug the TUI, since the screen is otherwise taken over by the UI.
- `--truncate-debug` starts that file fresh instead of appending.

Inside the TUI, external library output (client-go/klog) is captured and viewable
with ++f6++, and the developer console (++colon++) offers live inspection
commands.

## Recorder robustness

Long-lived watches routinely see transient failures: the apiserver or an
intermediary (konnectivity, a load balancer) periodically resets connections
(`INTERNAL_ERROR received from peer`), or briefly throttles re-lists (`the server
is currently unable to handle the request`, HTTP 503 from API Priority &
Fairness). client-go recovers by re-listing and re-watching, so **recording
continues** across these bursts, the pod stays healthy and segments keep
growing.

The recorder minimizes and surfaces them with three flags:

| Flag | Default | What it does |
|------|---------|--------------|
| `--disable-http2` | `true` | Puts each watch on its own HTTP/1.1 connection. |
| `--qps` | `20` | Client-side QPS limit toward the apiserver. |
| `--burst` | `30` | Client-side burst limit toward the apiserver. |

Why HTTP/1.1 by default: with HTTP/2, all of the recorder's watches share one
connection, so a single `GOAWAY` drops **every** watch at once and they all
re-list simultaneously, which trips API Priority & Fairness (the 503 storm).
One HTTP/1.1 connection per watch confines a reset to that single watch. Set
`--disable-http2=false` to go back to shared HTTP/2 (plus keepalive pings,
tunable with the `HTTP2_*` env vars above).

`--qps` / `--burst` raise client-side limits so that when a recovery re-list
does happen, it is not throttled to a crawl.

Watch and list failures are logged at WARN (one line per failed attempt) so you
can see exactly what is happening:

```text
WRN watch/list failed; client-go will retry with backoff gvr="apps/v1, Resource=deployments" error="..."
```

If you still see frequent bursts, it usually points at apiserver-side pressure
rather than loog. Narrowing the watched set, especially cluster-wide Pods,
reduces loog's load on the control plane.

## Operational limits

- **Single writer.** bbolt locks the active segment; run exactly one recorder
  per data directory (`replicas: 1` + `Recreate`). For HA, use leader election,
  not horizontal scaling.
- **Memory scales with what you watch.** Informers cache full objects in memory.
  High-cardinality kinds (Pods, Events, EndpointSlices) cluster-wide are
  expensive, start narrow, raise the container memory limit if you must.
- **Disk ≈ change volume × retention.** Guard it with `--retention` and/or
  `--max-segments`, and size the PVC accordingly.

## See also

- [CLI reference](cli.md)
- [Run the always-on recorder](../how-to/run-the-recorder.md)
- [Deploy the recorder to Kubernetes](../how-to/deploy-to-kubernetes.md)
