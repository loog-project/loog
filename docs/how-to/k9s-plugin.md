# Use loog with k9s

[k9s](https://github.com/derailed/k9s) supports custom plugins bound to
keystrokes. loog ships a `plugins.yaml` that adds shortcuts to launch loog on
the resource under your cursor.

## Before you begin

- loog [installed](../getting-started/installation.md) and on your `PATH`.
- k9s installed.
- A clone of the loog repository (for `compat/k9s/plugins.yaml`).

## Automatic (Makefile)

```bash
make link-k9s     # install the loog shortcuts for k9s
make unlink-k9s   # remove them
```

## Manual

Copy the shipped plugins file into your k9s plugins directory (or merge it into
an existing `plugins.yaml`):

=== "macOS"

    ```bash
    mkdir -p ~/Library/Application\ Support/k9s/plugins
    cp ./compat/k9s/plugins.yaml \
      ~/Library/Application\ Support/k9s/plugins/loog-plugins.yaml
    ```

=== "Linux"

    ```bash
    mkdir -p ~/.config/k9s/plugins
    cp ./compat/k9s/plugins.yaml ~/.config/k9s/plugins/loog-plugins.yaml
    ```

The exact k9s configuration directory can vary by version and OS; see the
[k9s configuration docs](https://github.com/derailed/k9s#k9s-configuration) if
these paths do not match your setup.

## Verify

Restart k9s, navigate to a resource view, and trigger the loog shortcut (k9s
lists active plugin keys in its `?` help). loog opens on the selected resource.

## Next steps

- [The terminal UI](../concepts/tui.md).
- [Use loog as a kubectl plugin](kubectl-plugin.md).
