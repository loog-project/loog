# Filter what gets recorded

The `-f/--filter` flag takes an [expr-lang](https://github.com/expr-lang/expr)
boolean expression. loog evaluates it for every event and only records objects
for which it returns `true`. The default is `All()` (record everything).

This guide is task-focused. For the full helper list and evaluation semantics,
see the [Filter expressions reference](../reference/filters.md) and
[The filtering model](../concepts/filtering.md).

## Before you begin

Decide *what* you want to keep. Filters are cheapest when they exclude
high-volume objects you do not care about (for example, everything outside one
namespace).

## Filter by namespace

```bash
# Only objects in prod and kube-system
loog -f 'Namespaces("prod","kube-system")' v1/pods
```

`Namespace(...)` is an alias for `Namespaces(...)`.

## Filter by name

```bash
# Only specific objects, cluster-wide
loog -f 'Names("nginx","api")' v1/services
```

## Pin a single object

```bash
loog -f 'Namespaced("default","nginx-deployment")' apps/v1/deployments
```

## Filter by label

```bash
# Objects carrying any of these label keys
loog -f 'LabelExists("app","tier")' v1/pods

# A specific key/value
loog -f 'Label("app","web")' apps/v1/deployments
```

## Combine with boolean logic

```bash
loog -f 'Namespaces("prod") && !Names("tmp","scratch")' v1/configmaps
```

## Match on the event or raw object

Two variables are available in every expression:

- `Event.Type` is one of `ADDED`, `MODIFIED`, `DELETED`.
- `Object` is the Kubernetes object as an `unstructured.Unstructured`, so its
  methods are available.

```bash
# Only live MODIFIED events in prod
loog -f 'Event.Type == "MODIFIED" && Namespace("prod")' v1/pods

# Reach into the object directly
loog -f 'Object.GetLabels()["app"] == "web" && Object.GetNamespace() == "adm"' \
  apps/v1/deployments
```

## Verify

Start loog and watch which objects appear. If the left panel stays empty on a
busy cluster, your filter is probably too strict, start with `All()` and tighten
from there.

## Troubleshooting

- **"error compiling filter expression"** — the expression is not valid
  expr-lang or does not return a boolean. Wrap comparisons so the whole thing
  evaluates to `true`/`false`.
- **Fewer results than expected** — remember filters run *before writing* live
  events, so anything filtered out is simply never stored.
- **Historic objects missing on load** — when loog pre-loads an existing file,
  the filter is applied to historic objects too (using `Object`; `Event` is not
  set there). Avoid depending on `Event.Type` if you also need historic matches.

!!! important "Recording vs. replay"
    While recording, a non-matching object is **not added to the database**.
    While replaying (`--replay`), the same flag is only a **view** and never
    changes the file.

## Next steps

- [Filter expressions reference](../reference/filters.md) — every helper and
  variable.
- [The filtering model](../concepts/filtering.md) — when the filter runs and why
  it matters.
