# Use loog as a kubectl plugin

`kubectl` treats any executable named `kubectl-<something>` on your `PATH` as a
subcommand. Linking loog under such a name lets you run it as
`kubectl <something>`.

## Before you begin

- loog [installed](../getting-started/installation.md) and on your `PATH`.

## Automatic (Makefile)

From a clone of the repository:

```bash
make link-kubectl     # install loog as a kubectl plugin
make unlink-kubectl   # remove it
```

## Manual

Link the loog binary to a `kubectl-`prefixed name in the same directory:

```bash
ln -s "$(which loog)" "$(dirname "$(which loog)")/kubectl-observe"
```

Now use it as a kubectl subcommand:

```bash
kubectl observe v1/configmaps
```

Everything after the subcommand name is passed straight to loog, so flags work
as usual:

```bash
kubectl observe -f 'Namespaces("prod")' apps/v1/deployments
```

## Verify

```bash
kubectl plugin list | grep kubectl-observe
kubectl observe --simulate
```

## Limitations

- Shell completions are not wired through the plugin name yet; use plain `loog`
  if you rely on completions.

## Next steps

- [Use loog with k9s](k9s-plugin.md).
- [Install shell completions](shell-completions.md) for the standalone binary.
