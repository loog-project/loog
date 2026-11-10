# Filter expressions

The `-f/--filter` flag (on `loog`, `loog record`, `loog extract`, and via the
`filter` query param on the HTTP API) takes a boolean
[expr-lang](https://github.com/expr-lang/expr) expression. The default is
`All()`.

For *when* the filter runs (gate vs. view), see
[The filtering model](../concepts/filtering.md). For task recipes, see
[Filter what gets recorded](../how-to/filter-resources.md).

## Built-in helpers

| Helper | Alias | Returns true when... |
|--------|-------|----------------------|
| `All()` | | always (record everything) |
| `Namespaces("a","b",...)` | `Namespace(...)` | the object is in one of the given namespaces |
| `Names("a","b",...)` | `Name(...)` | the object's name is one of the given names |
| `Namespaced("ns","name")` | | the object is namespaced with exactly this namespace and name |
| `LabelExists("k1","k2",...)` | | the object has **any** of the given label keys |
| `Label("key","value")` | | the object has a label `key` equal to `value` |

Helpers are variadic where it makes sense and can be combined with boolean
operators.

## Variables

| Variable | Type | Notes |
|----------|------|-------|
| `Object` | `unstructured.Unstructured` | The Kubernetes object. Its methods are callable. |
| `Event.Type` | string | `ADDED`, `MODIFIED`, or `DELETED`. Live events only. |

Commonly used `Object` methods:

```text
Object.GetName()            Object.GetNamespace()
Object.GetLabels()          Object.GetAnnotations()
Object.GetKind()            Object.GetAPIVersion()
Object.GetUID()             Object.GetResourceVersion()
```

## Operators

Standard expr-lang operators are available:

- Logical: `&&`, `||`, `!`
- Comparison: `==`, `!=`, `<`, `<=`, `>`, `>=`
- Membership: `in`, e.g. `Object.GetNamespace() in ["prod","stage"]`
- Indexing: `Object.GetLabels()["app"]`

## Examples

```bash
# Namespace scoping
loog -f 'Namespaces("prod","kube-system")' v1/pods

# By name, cluster-wide
loog -f 'Names("nginx","api")' v1/services

# One specific object
loog -f 'Namespaced("default","nginx-deployment")' apps/v1/deployments

# Label presence and value
loog -f 'LabelExists("app")' v1/pods
loog -f 'Label("tier","frontend")' apps/v1/deployments

# Boolean logic
loog -f 'Namespaces("prod") && !Names("tmp","scratch")' v1/configmaps

# Event-type gating (recording only)
loog -f 'Event.Type == "MODIFIED" && Namespace("prod")' v1/pods

# Reaching into the object
loog -f 'Object.GetLabels()["app"] == "web" && Object.GetNamespace() == "adm"' \
  apps/v1/deployments

# Membership
loog -f 'Object.GetNamespace() in ["prod","stage","canary"]' v1/pods
```

## Rules and gotchas

- The expression **must evaluate to a boolean**. A non-bool result is an error.
- A filter that fails to **compile** stops the command with
  `error compiling filter expression`.
- During recording the filter runs per event and can use `Event.Type`. During
  historic load and replay it runs per object using `Object` only, `Event` is
  not set there.
- Quote the whole expression in your shell (single quotes) so `&&`, `!`, and
  `"` are not interpreted by the shell.

## See also

- [The filtering model](../concepts/filtering.md)
- [Filter what gets recorded](../how-to/filter-resources.md)
- [expr-lang language definition](https://expr-lang.org/docs/language-definition)
