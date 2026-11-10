# Change analysis

Beyond storing revisions, loog derives a few things from a resource's revision
chain to help you spot patterns without reading every diff. This analysis is
pure computation over the recorded history, it adds nothing to the stored file.

## Change tags

Each revision is classified by comparing it to the previous one, so you can see
*what kind* of change it was at a glance:

| Tag | Meaning |
|-----|---------|
| `image` | a container image changed (matched by container name) |
| `replicas` | the `spec.replicas` value changed |
| `spec` | some other `spec` change |
| `status` | `status` changed |
| `labels` | `metadata.labels` changed |
| `config` | annotations or `data` (ConfigMap/Secret) changed |
| `unknown` | first revision, or no recognized change |

A revision can carry more than one tag. Image and replicas changes are detected
specifically because they are the ones you usually care about; everything else
in `spec` falls back to the generic `spec` tag.

## Hot resources

loog tracks how frequently each resource changes and flags the busy ones so they
stand out in the lists:

- `▲` — high change frequency ("hot").
- `△` — moderate change frequency ("warm").

This surfaces the handful of objects generating most of the churn, often the
first thing you want when a cluster feels noisy.

## Reconcile-loop detection

A controller stuck oscillating between states is a common, expensive bug. loog
detects it by fingerprinting recent object states within a sliding window: if
the *same* full state reappears, the resource is looping.

When a loop is found, the resource is marked with `↻` and loog computes:

- **Distinct states** in the loop (e.g. 2 for a simple A↔B flip).
- **Cycles** — how many complete oscillations occurred.
- **Period** — the average time for one full cycle.
- **Pattern sample** — a short label sequence like `A→B→A→B` from the tail of
  the window.

States are labelled `A`, `B`, ... `Z`, `AA`, ... so even many-state loops get
unique labels. Only states that recur (appear at least twice in the window)
count as loop participants, so a resource merely moving through new states is
not flagged.

!!! note "It compares full state"
    Detection marshals each revision's object and looks for exact repeats. Two
    "different-looking" revisions that reduce to the same object state are what
    a loop is, this is deliberately strict to avoid false positives from
    ordinary progressions.

## Burst grouping

Changes clustered tightly in time are bracketed together in the Timeline
(`╭ │ ╰`) so a flurry of rapid updates reads as one event rather than pushing
everything else off screen. When events land less than a second apart, loog
cannot rely on timestamps for order and falls back to `resourceVersion`,
indicated with a `≈` marker.

## See also

- [The terminal UI](tui.md) — where these indicators appear.
- [Revisions, snapshots and patches](revisions.md) — the data analysis runs on.
