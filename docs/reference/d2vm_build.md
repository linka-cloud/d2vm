## d2vm build

Build a vm image from Dockerfile

```
d2vm build [context directory] [flags]
```

### Options

```
      --append-to-cmdline string   Extra kernel cmdline arguments to append to the generated one
      --build-arg stringArray      Set build-time variables
  -f, --file string                Name of the Dockerfile
      --force                      Override output image
  -h, --help                       help for build
      --network-manager string     Network manager to use for the image: none, netplan, ifupdown
  -o, --output string              The output image, the extension determine the image format, raw will be used if none. Supported formats: qcow2 qed raw vdi vhd vmdk (default "disk0.qcow2")
  -p, --password string            Root user password (default "root")
  -s, --size string                The output image size (default "10G")
```

### Options inherited from parent commands

```
  -v, --verbose   Enable Verbose output
```

### SEE ALSO

* [d2vm](d2vm.md)	 - 

