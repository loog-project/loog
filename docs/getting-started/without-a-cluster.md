# Try it without a cluster

You do not need Kubernetes to explore the loog TUI. The `--simulate` flag runs
the interface on generated data, which is the fastest way to learn the keys and
see what the views look like when things are actually changing.

## Run the simulator

```bash
loog --simulate
```

This starts the TUI with a synthetic set of resources that mutate over time,
including some that oscillate so you can see [reconcile-loop
detection](../concepts/analysis.md) in action. Nothing connects to a cluster and
nothing is written to disk.

## What to try

- Press ++question++ to open the keyboard reference and skim the sections.
- Press ++ctrl+k++ for the command palette (fuzzy-searchable actions).
- Switch views with ++f1++ (Explorer), ++f2++ (Timeline), ++f3++ (Compare).
- In a resource's revision list, press ++c++ to mark a revision for compare,
  then mark a second one and open the Compare view.
- In the detail pane, cycle display modes: ++d++ diff, ++o++ full object, ++p++
  changes only, ++shift+j++ JSON, ++r++ raw database record.
- Open the developer console with ++colon++ and type `help` to see the built-in
  inspection commands (`status`, `resources`, `store`, `kinds`, ...).

!!! tip "Driving the simulation"
    From the developer console (++colon++), `sim start` and `sim stop` control
    the live data generator, so you can freeze the stream while you inspect a
    specific revision.

## Next steps

- Ready for a real cluster? Follow the [Quickstart](quick-start.md).
- Learn the interface in depth in [The terminal UI](../concepts/tui.md).
