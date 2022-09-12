## d2vm run qemu

Run the virtual machine image with qemu

```
d2vm run qemu [options] [image-path] [flags]
```

### Options

```
      --accel string            Choose acceleration mode. Use 'tcg' to disable it. (default "hvf:tcg")
      --arch string             Type of architecture to use, e.g. x86_64, aarch64, s390x (default "x86_64")
      --cpus uint               Number of CPUs (default 1)
      --data string             String of metadata to pass to VM
      --detached                Set qemu container to run in the background
      --device multiple-flag    Add USB host device(s). Format driver[,prop=value][,...] -- add device, like --device on the qemu command line. (default A multiple flag is a type of flag that can be repeated any number of times)
      --disk disk               Disk config, may be repeated. [file=]path[,size=1G][,format=qcow2] (default [])
      --gui                     Set qemu to use video output instead of stdio
  -h, --help                    help for qemu
      --mem uint                Amount of memory in MB (default 1024)
      --networking string       Networking mode. Valid options are 'default', 'user', 'bridge[,name]', tap[,name] and 'none'. 'user' uses QEMUs userspace networking. 'bridge' connects to a preexisting bridge. 'tap' uses a prexisting tap device. 'none' disables networking.` (default "user")
      --publish multiple-flag   Publish a vm's port(s) to the host (default []) (default A multiple flag is a type of flag that can be repeated any number of times)
      --qemu string             Path to the qemu binary (otherwise look in $PATH)
      --usb                     Enable USB controller
```

### Options inherited from parent commands

```
  -t, --time string   Enable formated timed output, valide formats: 'relative (rel | r)', 'full (f)' (default "none")
  -v, --verbose       Enable Verbose output
```

### SEE ALSO

* [d2vm run](d2vm_run.md)	 - Run the virtual machine image

