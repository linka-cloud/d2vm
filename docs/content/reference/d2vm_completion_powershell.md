## d2vm completion powershell

Generate the autocompletion script for powershell

### Synopsis

Generate the autocompletion script for powershell.

To load completions in your current shell session:

	d2vm completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.


```
d2vm completion powershell [flags]
```

### Options

```
  -h, --help              help for powershell
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --time string   Enable formated timed output, valide formats: 'relative (rel | r)', 'full (f)' (default "none")
  -v, --verbose       Enable Verbose output
```

### SEE ALSO

* [d2vm completion](d2vm_completion.md)	 - Generate the autocompletion script for the specified shell

