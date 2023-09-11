## d2vm build

Build a vm image from Dockerfile

```
d2vm build [context directory] [flags]
```

### Options

```
      --append-to-cmdline string   Extra kernel cmdline arguments to append to the generated one
      --boot-fs string             Filesystem to use for the boot partition, ext4 or fat32
      --boot-size uint             Size of the boot partition in MB (default 100)
      --bootloader string          Bootloader to use: syslinux, grub (default "syslinux")
      --build-arg stringArray      Set build-time variables
  -f, --file string                Name of the Dockerfile
      --force                      Override output qcow2 image
  -h, --help                       help for build
      --keep-cache                 Keep the images after the build
      --luks-password string       Password to use for the LUKS encrypted root partition. If not set, the root partition will not be encrypted
      --network-manager string     Network manager to use for the image: none, netplan, ifupdown
  -o, --output string              The output image, the extension determine the image format, raw will be used if none. Supported formats: qcow2 qed raw vdi vhd vmdk (default "disk0.qcow2")
  -p, --password string            Optional root user password
      --push                       Push the container disk image to the registry
      --raw                        Just convert the container to virtual machine image without installing anything more
  -s, --size string                The output image size (default "10G")
      --split-boot                 Split the boot partition from the root partition
  -t, --tag string                 Container disk Docker image tag
```

### Options inherited from parent commands

```
      --time string   Enable formated timed output, valide formats: 'relative (rel | r)', 'full (f)' (default "none")
  -v, --verbose       Enable Verbose output
```

### SEE ALSO

* [d2vm](d2vm.md)	 - 

