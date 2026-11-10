# The terminal UI

The TUI is built for scrubbing large revision histories, not just reading one
object. This page explains its layout and modes. For the exact keys, see
[Keyboard shortcuts](../reference/keybindings.md); press ++question++ any time
for the in-app reference.

## Layout

The screen is three panels wide. What they contain depends on the active view.
Focus a panel with ++1++ / ++2++ / ++3++, or cycle with ++tab++ /
++shift+tab++. Press ++f++ to fullscreen the focused panel.

The status bar shows recording state, the active view, and context hints.

## Views

Switch with the function keys (or ++alt+1++..++alt+3++ where F-keys are
intercepted by your terminal):

### Explorer (++f1++)

Resources grouped by kind on the left, the selected resource's revision list in
the middle, and a detail pane on the right. This is the "browse one thing over
time" view.

- ++shift+s++ toggles a starred-only filter.
- ++shift+o++ toggles sort order (name / time).

### Timeline (++f2++)

Every revision across all resources, newest first, a firehose of cluster change.
Good for "what changed around 14:32?".

- ++shift+s++ starred-only, ++shift+r++ reverse direction.
- ++w++ cycles a time window centered on the selected revision
  (all → ±15s → ±30s → ±1m → ±5m), to zoom into a burst.
- A `≈` marker means order was *inferred*: events less than a second apart,
  sorted by `resourceVersion`.

### Compare (++f3++)

Two revisions side by side as a highlighted YAML diff. Mark revisions with
++c++ in any list (`[C1]` / `[C2]`), then open Compare.

- ++tab++ switches the active pane; ++shift+x++ clears the selection.
- ++j++ / ++k++ and ++ctrl+d++ / ++ctrl+u++ scroll the diff.

## The detail pane

For the selected revision, cycle how it is shown:

| Key | Mode | Shows |
|-----|------|-------|
| ++d++ | Diff | YAML diff vs. the previous revision |
| ++o++ | Object | the full object YAML |
| ++p++ | Changes | only fields that changed vs. previous |
| ++shift+j++ | JSON | the object as JSON |
| ++r++ | Raw | the raw database record (snapshot/patch) |

Move between revisions with ++[++ / ++]++. Export the current YAML to a file
with ++e++, copy it to the clipboard with ++y++, or jump to this revision in the
Timeline with ++t++.

## Reading the symbols

The lists use compact glyphs (full legend in the
[keyboard reference](../reference/keybindings.md#symbols)):

- `●` / `○` — active / idle resource.
- `+` `~` `-` — Added / Modified / Deleted event. Deleted resources render
  struck through.
- `↻` — a [reconcile loop](analysis.md) was detected.
- `▲` / `△` — high / moderate change frequency ("hot" resources).
- `★` — starred.
- `╭ │ ╰` — a burst group (many changes clustered in time).

## Live controls

When recording live:

- ++shift+p++ pauses/resumes recording without leaving the UI.
- ++f5++ freezes the view so incoming changes do not move your selection.
- ++a++ toggles auto-scroll (follow the newest revision).
- ++shift+w++ opens the **Watch Manager** to add or remove watched resource
  kinds on the fly.

## Finding things

- ++slash++ filters the focused panel inline: type to dim non-matches, ++enter++
  to hide them, ++esc++ to clear.
- ++slash++++slash++ (two slashes) opens **Quick Jump**, a fuzzy finder across
  resources.
- ++ctrl+k++ opens the **command palette** of all actions.

## Debug tools

- ++f6++ opens the debug log viewer (captures client-go/klog output).
- ++colon++ opens a **developer console** with inspection commands (`status`,
  `resources`, `revisions <uid>`, `inspect <uid>`, `store`, `kinds`,
  `sim start`/`sim stop`, ...). Type `help` there for the list.

## Theming

The UI uses a Catppuccin Mocha-based dark palette with blue accents, matching
loog's brand. Event colors are consistent throughout: green for added, peach for
modified, red for deleted.

## See also

- [Keyboard shortcuts](../reference/keybindings.md)
- [Change analysis](analysis.md)
