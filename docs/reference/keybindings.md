# Keyboard shortcuts

The complete in-app reference is always available: press ++question++ inside the
TUI. This page mirrors it. Keys are case-sensitive (`s` and `S` differ).

## Global

| Key | Action |
|-----|--------|
| ++q++ / ++ctrl+c++ | Quit |
| ++ctrl+k++ | Command palette |
| ++question++ | Toggle help |
| ++slash++ | Inline filter (Enter apply, Esc clear) |
| ++slash++ ++slash++ | Quick Jump (fuzzy resource finder) |
| ++f1++–++f3++ | Switch view (Explorer / Timeline / Compare) |
| ++alt+1++–++alt+3++ | Switch view (terminal fallback for F-keys) |
| ++1++ / ++2++ / ++3++ | Focus panel |
| ++tab++ | Next panel |
| ++shift+tab++ | Previous panel |
| ++f++ | Toggle fullscreen |
| ++a++ | Toggle auto-scroll |
| ++w++ | Cycle time window around selected revision |
| ++shift+p++ | Pause / resume recording |
| ++f5++ / ++alt+5++ | Freeze / unfreeze view |
| ++shift+w++ | Watch Manager (add/remove resource types) |
| ++f6++ / ++alt+6++ | Debug log viewer |
| ++colon++ | Developer console |

## Inline filter (++slash++)

| Key | Action |
|-----|--------|
| ++slash++ | Activate filter on focused panel |
| type... | Preview: non-matches dimmed |
| ++enter++ | Apply: hide non-matches |
| ++esc++ | Clear filter and exit |

## Lists (Resources / Revisions / Timeline)

| Key | Action |
|-----|--------|
| ++j++ / ++k++ | Move down / up |
| ++g++ / ++shift+g++ | Go to top / bottom |
| ++ctrl+d++ / ++page-down++ | Page down |
| ++ctrl+u++ / ++page-up++ | Page up |
| ++enter++ / ++space++ | Select / expand |
| ++s++ | Toggle star |
| ++c++ | Mark for compare |

## Explorer view

| Key | Action |
|-----|--------|
| ++shift+s++ | Toggle starred-only filter |
| ++shift+o++ | Toggle sort order (name / time) |

## Timeline view

| Key | Action |
|-----|--------|
| ++shift+s++ | Toggle starred-only filter |
| ++shift+r++ | Reverse sort direction |
| ++w++ | Cycle time window around selected |
| `≈` | Order inferred (events <1s apart; sorted by resourceVersion) |

## Compare view

| Key | Action |
|-----|--------|
| ++tab++ | Switch left / right pane |
| ++shift+x++ | Clear compare selection |
| ++j++ / ++k++ | Scroll diff up / down |
| ++ctrl+d++ / ++page-down++ | Page down in diff |
| ++ctrl+u++ / ++page-up++ | Page up in diff |

## Detail view

| Key | Action |
|-----|--------|
| ++d++ | Diff mode (YAML with highlighting) |
| ++o++ | Object mode (full YAML) |
| ++p++ | Changes mode (only fields changed vs previous) |
| ++shift+j++ | JSON mode |
| ++r++ | Raw mode (database record) |
| ++bracket-left++ / ++bracket-right++ | Previous / next revision |
| ++e++ | Export YAML to file |
| ++y++ | Copy YAML to clipboard |
| ++t++ | Jump to timeline |

## Symbols

| Symbol | Meaning |
|--------|---------|
| `●` / `○` | Active / idle resource |
| `↻` | Reconcile loop detected |
| `▲` / `△` | High / moderate change frequency |
| `★` | Starred resource |
| `+` `~` `-` | Added / Modified / Deleted event |
| `[C1]` / `[C2]` | Compare mark left / right |
| `╭ │ ╰` | Burst group bracket |
| `▸` | Window anchor position |
| `▲` / `▼` | Scroll indicators |

## Developer console (++colon++)

| Command | Action |
|---------|--------|
| `help` | Show available commands |
| `status` | App state summary |
| `resources` | List all resources with UIDs |
| `revisions <uid>` | List revisions for a resource |
| `inspect <uid>` | Show raw revision data |
| `store` | Dump store statistics |
| `kinds` | List resource kinds and counts |
| `sim start` / `sim stop` | Start or stop live simulation |
| `log <msg>` | Write a debug log entry |
| `clear` | Clear console output |

## See also

- [The terminal UI](../concepts/tui.md) — what each view and mode does.
