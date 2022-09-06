## d2vm run hetzner

Run the virtual machine image on Hetzner Cloud

```
d2vm run hetzner [options] image-path [flags]
```

### Options

```
  -h, --help             help for hetzner
  -n, --name string      d2vm server name (default "d2vm")
      --rm               remove server when done
  -i, --ssh-key string   d2vm image identity key
  -t, --token string     Hetzner Cloud API token [$HETZNER_TOKEN]
  -u, --user string      d2vm image ssh user (default "root")
```

### Options inherited from parent commands

```
  -v, --verbose   Enable Verbose output
```

### SEE ALSO

* [d2vm run](d2vm_run.md)	 - Run the virtual machine image

