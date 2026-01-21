
# d2vm (Docker to Virtual Machine)

[![Language: Go](https://img.shields.io/badge/lang-Go-6ad7e5.svg?style=flat-square&logo=go)](https://golang.org/)
[![Go Reference](https://pkg.go.dev/badge/go.linka.cloud/d2vm.svg)](https://pkg.go.dev/go.linka.cloud/d2vm)
[![Chat](https://img.shields.io/badge/chat-matrix-blue.svg?style=flat-square&logo=matrix)](https://matrix.to/#/#d2vm:linka.cloud)

*Build virtual machine image from Docker images*

The project is heavily inspired by the [article](https://iximiuz.com/en/posts/from-docker-container-to-bootable-linux-disk-image/) and the work done by [iximiuz](https://github.com/iximiuz) on [docker-to-linux](https://github.com/iximiuz/docker-to-linux).

Many thanks to him.

**Status**: *alpha*

[![asciicast](https://asciinema.org/a/520132.svg)](https://asciinema.org/a/520132)

## Supported Environments:

**Only building Linux Virtual Machine images is supported.**

Starting from v0.1.0, **d2vm** automatically run build and convert commands inside Docker when not running on linux
or when running without *root* privileges.

*Note: windows should be working, but is totally untested.*

## Supported VM Linux distributions:

Working and tested:

- [x] Ubuntu (18.04+)
  Luks support is available only on Ubuntu 20.04+
- [x] Debian (stretch+)
  Luks support is available only on Debian buster+
- [x] Alpine
- [x] CentOS (8+)

Unsupported:

- [ ] RHEL

The program uses the `/etc/os-release` file to discover the Linux distribution and install the Kernel,
if the file is missing, the build cannot succeed.

Obviously, **Distroless** images are not supported.

## Prerequisites

### osx
- [Docker](https://docs.docker.com/get-docker/)
- [QEMU](https://www.qemu.org/download/#macos) (optional)
- [VirtualBox](https://www.virtualbox.org/wiki/Downloads) (optional)

### Linux
- [Docker](https://docs.docker.com/get-docker/)
- util-linux
- udev
- parted
- e2fsprogs
- dosfstools (when using fat32)
- mount
- tar
- extlinux (when using syslinux)
- qemu-utils
- cryptsetup (when using LUKS)
- [QEMU](https://www.qemu.org/download/#linux) (optional)
- [VirtualBox](https://www.virtualbox.org/wiki/Linux_Downloads) (optional)

#### sudo or root privileges

*sudo* or root privileges are required for `d2vm` to performs operations that require root-level permissions, in particular:

- mounting disk images and loopback devices requires [elevated privileges](https://linux.die.net/man/2/mount)
- invoke `docker` commands, which require root-level permissions by default


## Getting started

### Install

#### With Docker

*Note: this will only work if both the source context (and Dockerfile) and the output directory are somewhere inside
the directory where you run the command.*

```bash
docker pull linkacloud/d2vm:latest
alias d2vm="docker run --rm -it -v /var/run/docker.sock:/var/run/docker.sock --privileged -v \$PWD:/d2vm -w /d2vm linkacloud/d2vm:latest"
```

```bash
which d2vm

d2vm: aliased to docker run --rm -it -v /var/run/docker.sock:/var/run/docker.sock --privileged -v $PWD:/d2vm -w /d2vm linkacloud/d2vm:latest
```

#### With Homebrew

```bash
brew install linka-cloud/tap/d2vm
```

#### From release

Download the latest release for your platform from the [release page](https://github.com/linka-cloud/d2vm/releases/latest).

Extract the tarball, then move the extracted *d2vm* binary to somewhere in your `$PATH` (`/usr/local/bin` for most users).

```bash
VERSION=$(git ls-remote --tags https://github.com/linka-cloud/d2vm |cut -d'/' -f 3|tail -n 1)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$([ "$(uname -m)" = "x86_64" ] && echo "amd64" || echo "arm64")
curl -sL "https://github.com/linka-cloud/d2vm/releases/download/${VERSION}/d2vm_${VERSION}_${OS}_${ARCH}.tar.gz" | tar -xvz d2vm
sudo mv d2vm /usr/local/bin/
```

#### From source

Clone the git repository:

```bash
git clone https://github.com/linka-cloud/d2vm && cd d2vm
```

Install using the *make*, *docker* and the Go tool chain:

```bash
make install
```

The *d2vm* binary is installed in the `$GOBIN` directory.

```bash
which d2vm

/go/bin/d2vm
```

### Generate shell completion

The *d2vm* program supports shell completion for *bash*, *zsh* and *fish*.

It can be enabled by running the following command:

```bash
source <(d2vm completion $(basename $SHELL))
```

Or you can install the completion file in the shell completion directory by following the instructions:

```bash
d2vm completion $(basename $SHELL) --help
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
      --append-to-cmdline string   Extra kernel cmdline arguments to append to the generated one
      --boot-fs string             Filesystem to use for the boot partition, ext4 or fat32
      --boot-size uint             Size of the boot partition in MB (default 100)
      --bootloader string          Bootloader to use: syslinux, grub, grub-bios, grub-efi, defaults to syslinux on amd64 and grub-efi on arm64
      --force                      Override output qcow2 image
  -h, --help                       help for convert
      --hostname string            Hostname to set in the generated image (default "localhost")
      --keep-cache                 Keep the images after the build
      --luks-password string       Password to use for the LUKS encrypted root partition. If not set, the root partition will not be encrypted
      --network-manager string     Network manager to use for the image: none, netplan, ifupdown
  -o, --output string              The output image, the extension determine the image format, raw will be used if none. Supported formats: qcow2 qed raw vdi vhd vmdk (default "disk0.qcow2")
  -p, --password string            Optional root user password
      --platform string            Platform to use for the container disk image, linux/arm64 and linux/arm64 are supported (default "linux/amd64")
      --pull                       Always pull docker image
      --push                       Push the container disk image to the registry
      --raw                        Just convert the container to virtual machine image without installing anything more
  -s, --size string                The output image size (default "10G")
      --split-boot                 Split the boot partition from the root partition
  -t, --tag string                 Container disk Docker image tag

Global Flags:
      --time string   Enable formated timed output, valide formats: 'relative (rel | r)', 'full (f)' (default "none")
  -v, --verbose       Enable Verbose output

```

Create an image based on the **ubuntu** official image:

```bash
sudo d2vm convert ubuntu -o ubuntu.qcow2 -p MyP4Ssw0rd
```
```
Pulling image ubuntu
Inspecting image ubuntu
No network manager specified, using distribution defaults: netplan
Docker image based on Ubuntu 22.04.1 LTS (Jammy Jellyfish)
Building kernel enabled image
Creating vm image
Creating raw image
Mounting raw image
Creating raw image file system
Copying rootfs to raw image
Setting up rootfs
Installing linux kernel
Unmounting raw image
Writing MBR
Converting to qcow2
```

You can now run your ubuntu image using the created `ubuntu.qcow2` image with **qemu**:

```bash
d2vm run qemu ubuntu.qcow2
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

Type `poweroff` to shut down the vm.

### Building a VM Image from a Dockerfile

The example directory contains very minimalistic examples:

```bash
cd examples
```

*ubuntu.Dockerfile* :

```dockerfile
FROM ubuntu

RUN apt update && apt install -y openssh-server && \
    echo "PermitRootLogin yes" >> /etc/ssh/sshd_config

```

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
      --append-to-cmdline string   Extra kernel cmdline arguments to append to the generated one
      --boot-fs string             Filesystem to use for the boot partition, ext4 or fat32
      --boot-size uint             Size of the boot partition in MB (default 100)
      --bootloader string          Bootloader to use: syslinux, grub, grub-bios, grub-efi, defaults to syslinux on amd64 and grub-efi on arm64
      --build-arg stringArray      Set build-time variables
  -f, --file string                Name of the Dockerfile
      --force                      Override output qcow2 image
  -h, --help                       help for build
      --hostname string            Hostname to set in the generated image (default "localhost")
      --keep-cache                 Keep the images after the build
      --luks-password string       Password to use for the LUKS encrypted root partition. If not set, the root partition will not be encrypted
      --network-manager string     Network manager to use for the image: none, netplan, ifupdown
  -o, --output string              The output image, the extension determine the image format, raw will be used if none. Supported formats: qcow2 qed raw vdi vhd vmdk (default "disk0.qcow2")
  -p, --password string            Optional root user password
      --platform string            Platform to use for the container disk image, linux/arm64 and linux/arm64 are supported (default "linux/amd64")
      --pull                       Always pull docker image
      --push                       Push the container disk image to the registry
      --raw                        Just convert the container to virtual machine image without installing anything more
  -s, --size string                The output image size (default "10G")
      --split-boot                 Split the boot partition from the root partition
  -t, --tag string                 Container disk Docker image tag

Global Flags:
      --time string   Enable formated timed output, valide formats: 'relative (rel | r)', 'full (f)' (default "none")
  -v, --verbose       Enable Verbose output

```

```bash
sudo d2vm build -p MyP4Ssw0rd -f ubuntu.Dockerfile -o ubuntu.qcow2 .
```

Or if you want to create a VirtualBox image:

```bash
sudo d2vm build -p MyP4Ssw0rd -f ubuntu.Dockerfile -o ubuntu.vdi .
```

### KubeVirt Container Disk Images

Using the `--tag` flag with the `build` and `convert` commands, you can create a
[Container Disk Image](https://kubevirt.io/user-guide/virtual_machines/disks_and_volumes/#containerdisk) for [KubeVirt](https://kubevirt.io/).

The `--push` flag will push the image to the registry.

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
