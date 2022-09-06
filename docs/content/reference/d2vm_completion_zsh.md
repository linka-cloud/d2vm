## d2vm completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions for every new session, execute once:

#### Linux:

	d2vm completion zsh > "${fpath[1]}/_d2vm"

#### macOS:

	d2vm completion zsh > /usr/local/share/zsh/site-functions/_d2vm

You will need to start a new shell for this setup to take effect.


```
d2vm completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
  -v, --verbose   Enable Verbose output
```

### SEE ALSO

* [d2vm completion](d2vm_completion.md)	 - Generate the autocompletion script for the specified shell

