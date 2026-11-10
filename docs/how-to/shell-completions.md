# Install shell completions

loog generates completion scripts for `bash`, `zsh`, `fish`, and `powershell`
via the `loog completion` command. Completions cover subcommands, flags, and, on
supported shells, live GVR suggestions.

## Before you begin

- loog [installed](../getting-started/installation.md) and on your `PATH`.

## Load for the current session

The quickest way to try it, sources completions into your running shell:

=== "bash"

    ```bash
    source <(loog completion bash)
    ```

=== "zsh"

    ```bash
    source <(loog completion zsh)
    ```

=== "fish"

    ```bash
    loog completion fish | source
    ```

=== "powershell"

    ```powershell
    loog completion powershell | Out-String | Invoke-Expression
    ```

## Install permanently

Add the equivalent line to your shell startup file so completions load in every
new shell:

=== "bash"

    ```bash
    echo 'source <(loog completion bash)' >> ~/.bashrc
    ```

=== "zsh"

    ```bash
    echo 'source <(loog completion zsh)' >> ~/.zshrc
    ```

    If completions are not picked up, ensure `compinit` runs in your `~/.zshrc`:

    ```bash
    autoload -Uz compinit && compinit
    ```

=== "fish"

    ```bash
    loog completion fish > ~/.config/fish/completions/loog.fish
    ```

=== "powershell"

    ```powershell
    loog completion powershell >> $PROFILE
    ```

## Verify

Open a new shell and type `loog ` then press ++tab++. You should see subcommands
(`record`, `extract`, `fetch`, `completion`, `version`) and, after starting to
type a resource, GVR suggestions.

## Next steps

- [CLI reference](../reference/cli.md) for the full command surface.
