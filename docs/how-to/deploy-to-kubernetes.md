# Deploy the recorder to Kubernetes

This runs [`loog record`](run-the-recorder.md) in-cluster as a Deployment that
writes rotating segments to a PersistentVolume and serves the retrieval API. The
repository ships ready-to-edit manifests under
[`deploy/kubernetes/`](https://github.com/loog-project/loog/tree/main/deploy);
this guide explains what they contain and how to adapt them.

## Before you begin

- Cluster admin (or enough RBAC to create a ClusterRole and bind it).
- A default StorageClass, or a class name to pin in the PVC.
- The list of resources you want to record, and the API groups they belong to.

## The pieces

The manifests are plain YAML, apply them directly with `kubectl apply -f`
(no Kustomize or Helm required):

| File | What it creates |
|------|-----------------|
| `00-namespace.yaml` | The `loog` namespace. |
| `10-rbac.yaml` | A ServiceAccount and a **read-only** ClusterRole (`get`/`list`/`watch`) plus its binding. |
| `20-pvc.yaml` | A `ReadWriteOnce` PVC for the rolling segment window. |
| `30-recorder.yaml` | The `Deployment` (single replica, `Recreate`) and a `Service` on port 8080. |

## 1. Grant read access to what you record

The ClusterRole must cover every API group you plan to watch. Read-only is
enough:

```yaml title="10-rbac.yaml (rules excerpt)"
rules:
  - apiGroups: [""]
    resources: ["pods", "configmaps", "services"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments", "statefulsets", "daemonsets"]
    verbs: ["get", "list", "watch"]
  # Add each CRD group you record, e.g.:
  # - apiGroups: ["example.com"]
  #   resources: ["*"]
  #   verbs: ["get", "list", "watch"]
```

!!! note "loog never needs write access"
    The recorder only reads. If a watched kind is missing from the ClusterRole,
    that watch fails; add the group and re-apply.

## 2. Size the volume

Disk usage is roughly *change volume × retention*. Set the PVC request
accordingly (and optionally pin a `storageClassName`):

```yaml title="20-pvc.yaml (excerpt)"
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 10Gi
  # storageClassName: <your-storage-class>
```

## 3. Configure the recorder

Edit the container `image` and the `args` in `30-recorder.yaml`. The trailing
positional args are the GVRs to record; the flags above them control rotation,
retention, and the API:

```yaml title="30-recorder.yaml (container excerpt)"
image: ghcr.io/loog-project/loog:latest
args:
  - record
  - --dir=/data
  - --interval=10m       # roll a new segment every 10 minutes
  - --retention=720h     # keep 30 days
  - --http=:8080         # serve the retrieval API
  - --filter=All()
  # robustness defaults (see the reference)
  - --disable-http2=true
  - --qps=20
  - --burst=30
  # resources to record:
  - apps/v1/deployments
  - v1/pods
  - v1/configmaps
```

Key parts of the pod spec, and why:

- **`replicas: 1` + `strategy: Recreate`.** bbolt takes an exclusive lock on the
  active segment, so the recorder is a single writer. The old pod must release
  the PVC before a new one mounts it. Do **not** scale replicas; for HA use
  leader election.
- **`readOnlyRootFilesystem: true` with a `/tmp` `emptyDir`.** The root FS is
  read-only; the retrieval API writes each extract to a temp file before
  streaming it, so `/tmp` must be writable.
- **`runAsNonRoot` / dropped capabilities / `seccompProfile: RuntimeDefault`.**
  Standard hardening; the binary needs none of it removed.
- **Liveness/readiness probes hit `/healthz`.**
- **Memory limit.** Informers cache full objects in memory. Raise the limit if
  you watch high-cardinality kinds cluster-wide.

## 4. Apply

```bash
kubectl apply -f deploy/kubernetes/
kubectl -n loog rollout status deploy/loog-recorder
kubectl -n loog logs deploy/loog-recorder -f   # watch which GVRs bind
```

Applying a single changed file works too:

```bash
kubectl apply -f deploy/kubernetes/30-recorder.yaml
```

## Verify

```bash
# Pod is Running and Ready
kubectl -n loog get pods

# Segments are being written
kubectl -n loog exec deploy/loog-recorder -- ls -lh /data

# Retrieval API is reachable
kubectl -n loog port-forward svc/loog-recorder 8080:8080
curl -s localhost:8080/segments | jq
loog fetch --server localhost:8080 --since 30m --replay
```

## Testing a custom image

To try a build from your working tree, the repo includes a helper that compiles
inside a builder image, tags with the current git commit, and pushes:

```bash
deploy/build-and-push.sh
# override the registry/platform if needed:
LOOG_REGISTRY=my.registry/loog deploy/build-and-push.sh --platform linux/arm64
```

Then point the Deployment at the pushed tag:

```bash
kubectl -n loog set image deploy/loog-recorder recorder=my.registry/loog:<commit>
```

## Troubleshooting

- **Pod stuck `Pending` on the PVC** — no default StorageClass; set
  `storageClassName` in `20-pvc.yaml`.
- **New pod stuck `ContainerCreating` after a change** — the `Recreate` strategy
  means the old pod must fully terminate and release the `ReadWriteOnce` volume
  first. Wait for it, or delete the old pod.
- **A watch fails / a kind is not recorded** — add its API group to the
  ClusterRole. `loog record` also skips GVRs that are not installed or served
  (logged as warnings), so a broad list is safe.
- **`/healthz` failing** — the recorder only serves it when `--http` is set;
  make sure the flag and the probe port agree.

!!! warning "Keep the retrieval API internal"
    Reach it via `kubectl port-forward` or a cluster-internal client. It has no
    authentication and streams object state, do not put it behind an Ingress.

## Next steps

- [Retrieve a time range](retrieve-a-time-range.md).
- [Configuration and environment](../reference/configuration.md).
- [Segments, rotation and retention](../concepts/segments.md).
