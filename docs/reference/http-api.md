# Recorder HTTP API

When `loog record` runs with `--http <addr>`, it serves a small **read-only**
retrieval API. `loog fetch` is a thin client over it, but the endpoints are
plain HTTP, so `curl` works too.

!!! warning "Not for public exposure"
    The API has no authentication and streams cluster object state. Bind it to
    localhost or reach it through `kubectl port-forward`. Never put it behind an
    Ingress.

## Endpoints

### `GET /healthz`

Liveness/readiness probe. Returns `200 OK` with the body `ok`. Not access-logged.

```bash
curl -s localhost:8080/healthz
# ok
```

### `GET /segments`

Lists the segment files currently on disk, oldest first, as JSON. The active
(currently-written) segment is omitted, since it cannot be opened read-only.

```bash
curl -s localhost:8080/segments | jq
```

```json
{
  "segments": [
    {
      "path": "/data/loog-20260916T080000.000Z.loog",
      "name": "loog-20260916T080000.000Z.loog",
      "start": "2026-09-16T08:00:00Z",
      "end": "2026-09-16T09:00:00Z",
      "sizeBytes": 245760
    }
  ]
}
```

| Field | Meaning |
|-------|---------|
| `path` | Absolute path on the recorder. |
| `name` | File name. |
| `start` | Segment start time. |
| `end` | Start of the following segment (absent for the newest listed). |
| `sizeBytes` | File size. |

### `GET /extract`

Clips a time range into a single `.loog` file and streams it back as
`application/octet-stream`. This endpoint **does** include the active segment
(read through the recorder's own open handle), so there is no blind window.

Query parameters:

| Param | Meaning |
|-------|---------|
| `from` | Window start (RFC3339 or `YYYY-MM-DD HH:MM:SS`). Empty = beginning. |
| `to` | Window end. Empty = now. |
| `since` | Relative window ending now, e.g. `2h`, `30m`. Overrides `from`/`to`. |
| `filter` | expr-lang filter ([reference](filters.md)). Empty uses the recorder's own filter. |

Response headers:

| Header | Meaning |
|--------|---------|
| `Content-Disposition` | `attachment; filename="loog-<timestamp>.loog"` |
| `Content-Length` | Size of the returned file. |
| `X-Loog-Revisions` | Number of revisions written. |
| `X-Loog-Resources` | Number of distinct resources included. |

```bash
# Last 30 minutes, saved locally
curl -sS -o out.loog 'localhost:8080/extract?since=30m'

# A precise window, one namespace (URL-encode the filter)
curl -sS -o incident.loog \
  'localhost:8080/extract?from=2026-09-10T14:00:00Z&to=2026-09-10T15:00:00Z&filter=Namespaces(%22payments%22)'

loog --replay out.loog
```

Using the client is usually easier, it builds the URL and can open the result:

```bash
loog fetch --server localhost:8080 --since 30m --replay
```

## Status codes

| Code | When |
|------|------|
| `200` | Success. |
| `400` | Bad `from`/`to`/`since`, or a filter that fails to compile. |
| `405` | Method other than `GET`. |
| `500` | Internal error (listing segments, extraction, I/O). |

## Logging

Requests to `/segments` and `/extract` are access-logged on the recorder
(method, path, query, client, status, bytes, duration), and `/extract`
additionally logs the resolved time window, the filter, and the resulting
revision/resource counts. `/healthz` is not logged. View them with:

```bash
kubectl -n loog logs deploy/loog-recorder -f
```

## See also

- [Retrieve a time range](../how-to/retrieve-a-time-range.md)
- [Segments, rotation and retention](../concepts/segments.md)
