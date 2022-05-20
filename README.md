
# d2vm (Docker to Virtual Machine)

[![Language: Go](https://img.shields.io/badge/lang-Go-6ad7e5.svg?style=flat-square&logo=go)](https://golang.org/)
[![Go Reference](https://pkg.go.dev/badge/go.linka.cloud/d2vm.svg)](https://pkg.go.dev/go.linka.cloud/d2vm)
[![Chat](https://img.shields.io/badge/chat-matrix-blue.svg?style=flat-square&logo=matrix)](https://matrix.to/#/#d2vm:linka.cloud)

*Build virtual machine image from Docker images*

The project is heavily inspired by the [article](https://iximiuz.com/en/posts/from-docker-container-to-bootable-linux-disk-image/) and the work done by [iximiuz](https://github.com/iximiuz) on [docker-to-linux](https://github.com/iximiuz/docker-to-linux).

Many thanks to him.

**Status**: *alpha*

[![asciicast](https://asciinema.org/a/4WFKxaSNWTMPMeYbZWcSNm2nm.svg)](https://asciinema.org/a/4WFKxaSNWTMPMeYbZWcSNm2nm)

## Supported Environments:

**Only Linux is supported.**

If you want to run it on **OSX** or **Windows** (the last one is totally untested) you can do it using Docker:

```bash
alias d2vm='docker run --rm -i -t --privileged -v /var/run/docker.sock:/var/run/docker.sock -v $PWD:/build -w /build linkacloud/d2vm' 
```

**Starting from v0.1.0, d2vm automatically run build and convert commands inside Docker when not running on linux**.

## Supported VM Linux distributions:

Working and tested:

- [x] Ubuntu
- [x] Debian
- [x] Alpine
- [x] CentOS

Unsupported:

- [ ] RHEL

The program uses the `/etc/os-release` file to discover the Linux distribution and install the Kernel,
if the file is missing, the build cannot succeed.

Obviously, **Distroless** images are not supported. 

## Getting started

Clone the git repository:

```bash
git clone https://github.com/linka-cloud/d2vm && cd d2vm
```

Install using the Go tool chain:

```bash
go install ./cmd/d2vm
which d2vm
```
```
# Should be install in the $GOBIN directory
/go/bin/d2vm
```

Or use an alias to the **docker** image:

```bash
alias d2vm='docker run --rm -i -t --privileged -v /var/run/docker.sock:/var/run/docker.sock -v $PWD:/build -w /build linkacloud/d2vm'
which d2vm
```
```
d2vm: aliased to docker run --rm -i -t --privileged -v /var/run/docker.sock:/var/run/docker.sock -v $PWD:/build -w /build linkacloud/d2vm
```

### Converting an existing Docker Image to VM image:

```bash
d2vm convert --help
```
```
Convert Docker image to vm image

Usage:
  d2vm convert [docker image] [flags]

Flags:
  -d, --debug             Enable Debug output
  -f, --force             Override output qcow2 image
  -h, --help              help for convert
  -o, --output string     The output image, the extension determine the image format. Supported formats: qcow2 qed raw vdi vhd vmdk (default "disk0.qcow2")
  -p, --password string   The Root user password (default "root")
      --pull              Always pull docker image
  -s, --size string       The output image size (default "10G")

```

Create an image based on the **ubuntu** official image:

```bash
sudo d2vm convert ubuntu -o ubuntu.qcow2 -p MyP4Ssw0rd
```
```
INFO[0000] pulling image ubuntu                         
INFO[0001] inspecting image ubuntu                      
INFO[0002] docker image based on Ubuntu                 
INFO[0002] building kernel enabled image                
INFO[0038] creating root file system archive            
INFO[0040] creating vm image                            
INFO[0040] creating raw image                           
INFO[0040] mounting raw image                           
INFO[0040] creating raw image file system               
INFO[0040] copying rootfs to raw image                  
INFO[0041] setting up rootfs                            
INFO[0041] installing linux kernel                      
INFO[0042] unmounting raw image                         
INFO[0042] writing MBR                                  
INFO[0042] converting to qcow2
```

You can now run your ubuntu image using the created `ubuntu.qcow2` image with **qemu**:

```bash
./qemu.sh ununtu.qcow2
```
```
SeaBIOS (version 1.13.0-1ubuntu1.1)


iPXE (http://ipxe.org) 00:03.0 CA00 PCI2.10 PnP PMM+BFF8C920+BFECC920 CA00



Booting from Hard Disk...

SYSLINUX 6.04 EDD 20191223 Copyright (C) 1994-2015 H. Peter Anvin et al
Now booting the kernel from SYSLINUX...
Loading /boot/vmlinuz... ok
Loading /boot/initrd.img...ok
[    0.000000] Linux version 5.4.0-109-generic (buildd@ubuntu) (gcc version 9)
[    0.000000] Command line: BOOT_IMAGE=/boot/vmlinuz ro root=UUID=b117d206-b8
[    0.000000] KERNEL supported cpus:
[    0.000000]   Intel GenuineIntel
[    0.000000]   AMD AuthenticAMD
[    0.000000]   Hygon HygonGenuine
[    0.000000]   Centaur CentaurHauls
[    0.000000]   zhaoxin   Shanghai

...

Welcome to Ubuntu 20.04.4 LTS!

[    3.610631] systemd[1]: Set hostname to <localhost>.
[    3.838984] systemd[1]: Created slice system-getty.slice.
[  OK  ] Created slice system-getty.slice.
[    3.845038] systemd[1]: Created slice system-modprobe.slice.
[  OK  ] Created slice system-modprobe.slice.
[    3.852054] systemd[1]: Created slice system-serial\x2dgetty.slice.
[  OK  ] Created slice system-serial\x2dgetty.slice.

...

Ubuntu 20.04.4 LTS localhost ttyS0

localhost login: 
```

Log in using the *root* user and the password configured at build time.

```
localhost login: root
Password:


Welcome to Ubuntu 20.04.4 LTS (GNU/Linux 5.4.0-109-generic x86_64)

 * Documentation:  https://help.ubuntu.com
 * Management:     https://landscape.canonical.com
 * Support:        https://ubuntu.com/advantage

This system has been minimized by removing packages and content that are
not required on a system that users do not log into.

To restore this content, you can run the 'unminimize' command.

The programs included with the Ubuntu system are free software;
the exact distribution terms for each program are described in the
individual files in /usr/share/doc/*/copyright.

Ubuntu comes with ABSOLUTELY NO WARRANTY, to the extent permitted by
applicable law.

root@localhost:~#
```

Type `poweroff` to shutdown the vm.

### Building a VM Image from a Dockerfile

The example directory contains very minimalistic examples:

```bash
cd examples
```

*ubuntu.Dockerfile* :

```dockerfile
FROM ubuntu

RUN apt update && apt install -y openssh-server && \
    echo "PermitRootLogin yes" >> /etc/ssh/sshd_config \

```

When building the vm image, *d2vm* will create a root password, so there is no need to configure it now.

Build the vm image:

The *build* command take most of its flags and arguments from the *docker build* command.

```bash
d2vm build --help
```

```
Build a vm image from Dockerfile

Usage:
  d2vm build [context directory] [flags]

Flags:
      --build-arg stringArray   Set build-time variables
  -d, --debug                   Enable Debug output
  -f, --file string             Name of the Dockerfile
      --force                   Override output image
  -h, --help                    help for build
  -o, --output string           The output image, the extension determine the image format. Supported formats: qcow2 qed raw vdi vhd vmdk (default "disk0.qcow2")
  -p, --password string         Root user password (default "root")
  -s, --size string             The output image size (default "10G")

```

```bash
sudo d2vm build -p MyP4Ssw0rd -f ubuntu.Dockerfile -o ubuntu.qcow2 .
```

Or if you want to create a VirtualBox image:

```bash
sudo d2vm build -p MyP4Ssw0rd -f ubuntu.Dockerfile -o ubuntu.vdi .
```

### Complete example

A complete example setting up a ZSH workstation is available in the [examples/full](examples/full/README.md) directory.


### Internal Dockerfile templates

You can find the Dockerfiles used to install the Kernel in the [templates](templates) directory.

### TODO / Questions:

- [ ] Create service from `ENTRYPOINT` `CMD` `WORKDIR` and `ENV` instructions ?
- [ ] Inject Image `ENV` variables into `.bashrc` or other service environment file ?
- [x] Use image layers to create *rootfs* instead of container ?

### Acknowledgments

The *run* commands are adapted from [linuxkit](https://github.com/docker/linuxkit).
