## d2vm run vbox

Run the virtual machine image with Virtualbox

```
d2vm run vbox [options] image-path [flags]
```

### Options

```
      --cpus uint               Number of CPUs (default 1)
      --disk disk               Disk config, may be repeated. [file=]path[,size=1G][,format=raw] (default [])
      --gui                     Show the VM GUI
  -h, --help                    help for vbox
      --mem uint                Amount of memory in MB (default 1024)
      --name string             Name of the Virtualbox VM
      --networking vbnetworks   Network config, may be repeated. [type=](null|nat|bridged|intnet|hostonly|generic|natnetwork[<devicename>])[,[bridge|host]adapter=<interface>] (default [])
      --vboxmanage string       VBoxManage binary to use (default "VBoxManage")
```

### Options inherited from parent commands

```
  -v, --verbose   Enable Verbose output
```

### SEE ALSO

* [d2vm run](d2vm_run.md)	 - Run the virtual machine image

