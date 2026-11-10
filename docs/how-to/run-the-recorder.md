# Run the always-on recorder

`loog record` is a headless, continuous collector. It watches your resources and
writes every change into **rotating `.loog` segments**: files that roll on a
fixed interval and are pruned after a retention window. The result is a
fixed-size, always-available rolling history, a "dashcam" for your cluster.

This guide runs the recorder locally. To run it in-cluster as a Deployment, see
[Deploy the recorder to Kubernetes](deploy-to-kubernetes.md).

## Before you begin

- loog [installed](../getting-started/installation.md).
- A writable directory for segments (`--dir`).
- A kubeconfig that can `get`/`list`/`watch` the resources you record.

## Record locally

```bash
loog record --dir ./segments --interval 1h --retention 720h \
  apps/v1/deployments v1/configmaps
```

- `--interval 1h` — roll to a fresh segment every hour.
- `--retention 720h` — delete segments whose window ends more than 30 days ago
  (`0` keeps everything).
- Each segment is **self-contained**: on rotation the tracker cache is reset, so
  the first change to any object in a new segment is written as a full snapshot.
  This is why a time range can be served by opening only the overlapping files.

Leave it running. It logs each rotation and prune.

## Serve the retrieval API

Add `--http` so `loog fetch` can pull time ranges over HTTP:

```bash
loog record --dir ./segments --http :8080 apps/v1/deployments
```

The API is read-only and intended for local/port-forward use. See the
[Recorder HTTP API reference](../reference/http-api.md).

!!! warning "Do not expose the retrieval API publicly"
    It streams cluster object state with no authentication. Bind it to localhost
    or reach it through a `kubectl port-forward`, never an Ingress.

## Cap disk usage

Two independent guards, use either or both:

- `--retention 720h` — time-based (delete anything older than the window).
- `--max-segments 500` — count-based hard cap (oldest deleted first).

Disk usage is roughly *change volume × retention*. High-churn kinds (Pods,
Events, EndpointSlices) can dominate; scope your watches and set a cap.

## Robustness on long-lived watches

Long watches see transient resets and throttling; client-go recovers by
re-listing, so recording continues. The recorder ships sensible defaults:

- `--disable-http2` (default `true`) puts each watch on its own HTTP/1.1
  connection, so a single apiserver `GOAWAY` does not drop every watch at once.
- `--qps` / `--burst` (default `20` / `30`) raise client-side limits so recovery
  re-lists are not throttled.

See [Configuration and environment](../reference/configuration.md#recorder-robustness)
for when to change these.

## Verify

```bash
# Segments accumulate in the directory
ls -lh ./segments
# loog-20260916T090000.000Z.loog
# loog-20260916T100000.000Z.loog

# If --http is on, list what is available
curl -s localhost:8080/segments | jq
```

Then pull a window and browse it:

```bash
loog fetch --server localhost:8080 --since 1h --replay
# or, from the files directly:
loog extract --dir ./segments --since 1h -o out.loog --replay
```

## Troubleshooting

- **A resource is not being recorded** — `loog record` skips any GVR that is not
  installed or whose version is not served (logged as a warning), so a broad
  list is safe. Check the startup logs for which GVRs bound.
- **"no changes in the last N minutes" when fetching** — over HTTP, `loog fetch`
  reads the active (still-open) segment too, so there is no blind window. When
  running `loog extract` against a **copied/offline** directory, only rolled
  segments are readable (bbolt locks the active file); prefer `fetch` against a
  live recorder for the freshest data.
- **Frequent watch-error warnings** — usually apiserver-side pressure (API
  Priority & Fairness, control-plane rollouts). Narrow the watched set,
  especially cluster-wide Pods.

## Next steps

- [Retrieve a time range](retrieve-a-time-range.md).
- [Deploy the recorder to Kubernetes](deploy-to-kubernetes.md).
- [Segments, rotation and retention](../concepts/segments.md).
