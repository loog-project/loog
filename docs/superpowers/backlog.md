# Backlog / known issues

Durable list of deferred fixes and follow-ups. Newest first. Not part of the
published docs (`mkdocs.yml` excludes `superpowers/`).

These came out of a code review that measured loog against Talmi's conventions
(`AGENTS.md`, `CONTRIBUTING.md`) and the Go skills. The theme the maintainer
wants to drive toward: **bring the loog CLI much closer to the Talmi CLI**, in
both shape (`newXCmd(deps)` constructors, central error rendering, output
format) and test coverage.

---

## CLI: align structure and hygiene with Talmi

**Found:** 2026-09-16 (review against Talmi).

This is the umbrella item the maintainer called out. Talmi's `cmd/` package is
the target shape:

- Every command is built by a `newXCmd(deps Deps) *cobra.Command` constructor
  (`newAuditCmd`, `newLeaseIssueCmd`, ...) instead of package-level `var xCmd`
  globals wired in `init()`. loog currently uses global `*cobra.Command` vars
  plus package-level flag variables (`cmd/root.go:40`, `cmd/record.go:32`) and
  `init()` registration. Move to constructors that take a `Deps`/`IOStreams`
  struct so flags are locals, state is injected, and commands are unit-testable
  in isolation.
- `rootCmd.SilenceUsage = true` and `rootCmd.SilenceErrors = true`
  (Talmi `cmd/root.go:72-73`). loog sets neither, so any `RunE` error dumps the
  full usage text. Render errors centrally in `Execute()` instead of a bare
  `os.Exit(1)` (`cmd/root.go:133`).
- Commands return a typed exit error and let a single top-level renderer decide
  the exit code, rather than `os.Exit` inside command bodies.
- CLI output format: follow Talmi's rule of picking the shape to fit the data,
  add a `--json`/`--output` flag for machine-readable output where loog prints
  human tables today.
- `IOStreams`-style dependency injection over globals (Talmi code-style rule):
  no package-level mutable state, pass stdout/stderr/logger in.

**Fix direction:** its own branch. Refactor `cmd/` to `newXCmd(deps)` +
`Deps`/`IOStreams`, set the silences, centralize error rendering, then backfill
command-level tests (see the testing item). Mechanical but touches every file
in `cmd/`; agree on the `Deps` shape first.

## Tests: adopt Talmi's testing conventions

**Found:** 2026-09-16.

Talmi's `AGENTS.md` testing section is the target:

- **`goleak`**: no goroutine-leak detection anywhere (`goleak`: 0 refs). loog is
  goroutine-heavy (`pkg/mux` dispatch, the tracker lock-janitor, the recorder
  loop, the TUI). Add package-level `TestMain` + `goleak.VerifyTestMain(m)` to
  the concurrent packages, starting with `pkg/mux` and `internal/service`.
- **`t.Parallel()`**: not used in any of the ~30 test files. Add it wherever
  safe, especially in table-driven subtests.
- **testify**: Talmi standardizes on `is := assert.New(t)` / `must :=
  require.New(t)`. loog uses the stdlib `testing` package by hand. This is a
  project choice, not a defect (the skills treat testify as optional), so decide
  deliberately: either adopt testify to match Talmi, or document that loog
  stays stdlib-only. Don't leave it accidental.
- **`-race`**: already run via `make test-race`; keep it in CI for the
  concurrent packages.

**Fix direction:** land `goleak` + `t.Parallel()` first (small, high value).
Fold command-level tests in alongside the CLI refactor above.

## HTTP recorder API has no server timeouts

**Found:** 2026-09-16.

**Symptom:** `&http.Server{Addr: recordHTTPAddr, Handler: api.Handler()}`
(`cmd/record.go:177`) sets no `ReadHeaderTimeout`/`ReadTimeout`/`WriteTimeout`/
`IdleTimeout`, so it is exposed to slow-client (Slowloris) stalls. `gosec` flags
this as G112. Severity is reduced because the API is documented as
port-forward-only and read-only (`internal/httpapi/server.go:5`), but it is a
cheap fix and defense in depth.

**Fix direction:** set at least `ReadHeaderTimeout` (and sensible read/write
timeouts) on the `http.Server`. Add `gosec` to the linter set so this class of
issue is caught automatically (see linter item).

## Linter config is thin vs the recommended set

**Found:** 2026-09-16.

**Symptom:** `.golangci.yml` runs the standard linters plus `bodyclose`,
`misspell`, `unconvert`, with `gofmt`+`goimports`. Talmi runs `gofumpt -extra`
plus ~33 linters with `errcheck.check-type-assertions: true`. The gap is why the
HTTP-timeout and missing-`t.Parallel()` issues went unflagged.

**Fix direction:** widen `.golangci.yml` toward the golang-lint recommended set.
Highest value additions: `gosec` (catches the HTTP timeout), `paralleltest`
(the `t.Parallel()` gap), `errcheck` type-assertion checking, and switching the
formatter to `gofumpt`. Enable incrementally with `issues.new-from-rev` if the
backlog of findings is large, then clean up.

## Performance: O(n^2) ingestion path in LiveStore

**Found:** 2026-09-16.

**Symptom:** the timeline prepend grows the slice then
`copy(s.timeline[1:], s.timeline)` on every event (`internal/adapter/livestore.go:78`
and `:324`), and `appendRevision` copies the whole revisions slice per revision
(`:393`). Both are O(n) per event, so O(n^2) over a session, on a path the
comment calls "high-throughput ingestion".

**Fix direction:** benchmark first (golang-benchmark). Likely: append to the
timeline and sort/reverse on read instead of prepending, and use an
amortized-growth structure for per-resource revisions so ingestion is amortized
O(1). Keep the copy-on-write reads that hand data to the TUI.

## Minor / low-severity cleanups

**Found:** 2026-09-16. Batch these into a single cleanup pass.

- **Typed atomics.** `lockWrap.lastUse int64` with `atomic.LoadInt64`/`StoreInt64`
  (`internal/service/tracker_service.go:38`); switch to `atomic.Int64`
  (Go 1.19+).
- **`signal.NotifyContext`.** `cmd/record.go:188` and `cmd/root.go:423` use
  `signal.Notify` + a manual channel; prefer `signal.NotifyContext`.
- **`slices.SortFunc` over `sort.Slice`** (`internal/store/segment/segment.go:327`).
- **Nil-guard on segment delegation.** `segment.Store` methods deref `s.active`
  without a nil check after `Close` sets it nil (`segment.go:212`-`244`); only
  `Close` guards. Lifecycle-controlled, so low risk.
- **viper env prefix.** `viper.AutomaticEnv()` runs without `SetEnvPrefix`
  (`cmd/root.go:153`); namespace env vars with a `LOOG_` prefix to avoid
  collisions.
</content>
</invoke>
