// Copyright 2022 Linka Cloud  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package d2vm

import (
	"context"
	"fmt"
	"os"
	exec2 "os/exec"
	"path/filepath"
	"strings"

	"github.com/c2h5oh/datasize"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"

	"go.linka.cloud/d2vm/pkg/exec"
)

const (
	hosts = `127.0.0.1 localhost

# The following lines are desirable for IPv6 capable hosts
::1 ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
ff02::3 ip6-allhosts
`
	syslinuxCfgUbuntu = `DEFAULT linux
  SAY Now booting the kernel from SYSLINUX...
 LABEL linux
  KERNEL /boot/vmlinuz
  APPEND ro root=UUID=%s initrd=/boot/initrd.img net.ifnames=0 console=tty0 console=ttyS0,115200n8 %s
`
	syslinuxCfgDebian = `DEFAULT linux
  SAY Now booting the kernel from SYSLINUX...
 LABEL linux
  KERNEL /vmlinuz
  APPEND ro root=UUID=%s initrd=/initrd.img net.ifnames=0 console=tty0 console=ttyS0,115200n8 %s
`
	syslinuxCfgAlpine = `DEFAULT linux
  SAY Now booting the kernel from SYSLINUX...
 LABEL linux
  KERNEL /boot/vmlinuz-virt
  APPEND ro root=UUID=%s rootfstype=ext4 initrd=/boot/initramfs-virt console=ttyS0,115200 %s
`
	syslinuxCfgCentOS = `DEFAULT linux
  SAY Now booting the kernel from SYSLINUX...
 LABEL linux
  KERNEL /boot/vmlinuz
  APPEND ro root=UUID=%s initrd=/boot/initrd.img net.ifnames=0 console=tty0 console=ttyS0,115200n8 %s
`
)

var (
	formats = []string{"qcow2", "qed", "raw", "vdi", "vhd", "vmdk"}

	mbrPaths = []string{
		// debian path
		"/usr/lib/syslinux/mbr/mbr.bin",
		// ubuntu path
		"/usr/lib/EXTLINUX/mbr.bin",
		// alpine path
		"/usr/share/syslinux/mbr.bin",
		// centos path
		"/usr/share/syslinux/mbr.bin",
		// archlinux path
		"/usr/lib/syslinux/bios/mbr.bin",
	}
)

const (
	perm os.FileMode = 0644
)

func sysconfig(osRelease OSRelease) (string, error) {
	switch osRelease.ID {
	case ReleaseUbuntu:
		if osRelease.VersionID < "20.04" {
			return syslinuxCfgDebian, nil
		}
		return syslinuxCfgUbuntu, nil
	case ReleaseDebian:
		return syslinuxCfgDebian, nil
	case ReleaseKali:
		return syslinuxCfgDebian, nil
	case ReleaseAlpine:
		return syslinuxCfgAlpine, nil
	case ReleaseCentOS:
		return syslinuxCfgCentOS, nil
	default:
		return "", fmt.Errorf("%s: distribution not supported", osRelease.ID)
	}
}

type Builder interface {
	Build(ctx context.Context) (err error)
	Close() error
}

type builder struct {
	osRelease OSRelease

	src     string
	img     *image
	diskRaw string
	diskOut string
	format  string

	size     uint64
	mntPoint string

	splitBoot bool
	bootSize  uint64

	mbrPath string

	loDevice        string
	bootPart        string
	rootPart        string
	cryptPart       string
	cryptRoot       string
	mappedCryptRoot string
	bootUUID        string
	rootUUID        string
	cryptUUID       string

	luksPassword string

	cmdLineExtra string
}

func NewBuilder(ctx context.Context, workdir, imgTag, disk string, size uint64, osRelease OSRelease, format string, cmdLineExtra string, splitBoot bool, bootSize uint64, luksPassword string) (Builder, error) {
	if err := checkDependencies(); err != nil {
		return nil, err
	}
	if luksPassword != "" {
		// TODO(adphi): remove this check when we support luks encryption on other distros
		if osRelease.ID == ReleaseCentOS {
			return nil, fmt.Errorf("luks encryption is not supported on centos")
		}
		if !splitBoot {
			return nil, fmt.Errorf("luks encryption requires split boot")
		}
	}
	f := strings.ToLower(format)
	valid := false
	for _, v := range formats {
		if valid = v == f; valid {
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("invalid format: %s valid formats are: %s", f, strings.Join(formats, " "))
	}

	if splitBoot && bootSize < 50 {
		return nil, fmt.Errorf("boot partition size must be at least 50MiB")
	}

	if splitBoot && bootSize >= size {
		return nil, fmt.Errorf("boot partition size must be less than the disk size")
	}

	mbrBin := ""
	for _, v := range mbrPaths {
		if _, err := os.Stat(v); err == nil {
			mbrBin = v
			break
		}
	}
	if mbrBin == "" {
		return nil, fmt.Errorf("unable to find syslinux's mbr.bin path")
	}
	if size == 0 {
		size = 10 * uint64(datasize.GB)
	}
	if disk == "" {
		disk = "disk0"
	}
	img, err := NewImage(ctx, imgTag, workdir)
	if err != nil {
		return nil, err
	}
	// i, err := os.Stat(imgTar)
	// if err != nil {
	// 	return nil, err
	// }
	// if i.Size() > size {
	// 	s := datasize.ByteSize(math.Ceil(datasize.ByteSize(i.Size()).GBytes())) * datasize.GB
	// 	logrus.Warnf("%s is smaller than rootfs size, using %s", datasize.ByteSize(size), s)
	// 	size = int64(s)
	// }
	b := &builder{
		osRelease:    osRelease,
		img:          img,
		diskRaw:      filepath.Join(workdir, disk+".d2vm.raw"),
		diskOut:      filepath.Join(workdir, disk+"."+format),
		format:       f,
		size:         size,
		mbrPath:      mbrBin,
		mntPoint:     filepath.Join(workdir, "/mnt"),
		cmdLineExtra: cmdLineExtra,
		splitBoot:    splitBoot,
		bootSize:     bootSize,
		luksPassword: luksPassword,
	}
	if err := os.MkdirAll(b.mntPoint, os.ModePerm); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *builder) Build(ctx context.Context) (err error) {
	defer func() {
		if err == nil {
			return
		}
		logrus.WithError(err).Error("Build failed")
		if err := b.unmountImg(context.Background()); err != nil {
			logrus.WithError(err).Error("failed to unmount")
		}
		if err := b.cleanUp(context.Background()); err != nil {
			logrus.WithError(err).Error("failed to cleanup")
		}
	}()
	if err = b.cleanUp(ctx); err != nil {
		return err
	}
	if err = b.makeImg(ctx); err != nil {
		return err
	}
	if err = b.mountImg(ctx); err != nil {
		return err
	}
	if err = b.copyRootFS(ctx); err != nil {
		return err
	}
	if err = b.setupRootFS(ctx); err != nil {
		return err
	}
	if err = b.installKernel(ctx); err != nil {
		return err
	}
	if err = b.unmountImg(ctx); err != nil {
		return err
	}
	if err = b.setupMBR(ctx); err != nil {
		return err
	}
	if err = b.convert2Img(ctx); err != nil {
		return err
	}
	if err = b.cleanUp(ctx); err != nil {
		return err
	}
	return nil
}

func (b *builder) cleanUp(ctx context.Context) error {
	return os.RemoveAll(b.diskRaw)
}

func (b *builder) makeImg(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	logrus.Infof("creating raw image")
	if err := block(b.diskRaw, b.size); err != nil {
		return err
	}

	var args []string
	if b.splitBoot {
		args = []string{"-s", b.diskRaw,
			"mklabel", "msdos", "mkpart", "primary", "1Mib", fmt.Sprintf("%dMib", b.bootSize),
			"mkpart", "primary", fmt.Sprintf("%dMib", b.bootSize), "100%",
			"set", "1", "boot", "on",
		}
	} else {
		args = []string{"-s", b.diskRaw, "mklabel", "msdos", "mkpart", "primary", "1Mib", "100%", "set", "1", "boot", "on"}
	}

	if err := exec.Run(ctx, "parted", args...); err != nil {
		return err
	}

	return nil
}

func (b *builder) mountImg(ctx context.Context) error {
	logrus.Infof("mounting raw image")
	o, _, err := exec.RunOut(ctx, "losetup", "--show", "-f", b.diskRaw)
	if err != nil {
		return err
	}
	b.loDevice = strings.TrimSuffix(o, "\n")
	if err := exec.Run(ctx, "kpartx", "-a", b.loDevice); err != nil {
		return err
	}
	b.bootPart = fmt.Sprintf("/dev/mapper/%sp1", filepath.Base(b.loDevice))
	if b.splitBoot {
		b.rootPart = fmt.Sprintf("/dev/mapper/%sp2", filepath.Base(b.loDevice))
	} else {
		b.rootPart = b.bootPart
	}
	if b.isLuksEnabled() {
		logrus.Infof("encrypting root partition")
		f, err := os.CreateTemp("", "key")
		if err != nil {
			return err
		}
		defer f.Close()
		defer os.Remove(f.Name())
		if _, err := f.WriteString(b.luksPassword); err != nil {
			return err
		}
		// cryptsetup luksFormat --batch-mode --verify-passphrase --type luks2 $ROOT_DEVICE $KEY_FILE
		if err := exec.Run(ctx, "cryptsetup", "luksFormat", "--batch-mode", "--type", "luks2", b.rootPart, f.Name()); err != nil {
			return err
		}
		b.cryptRoot = fmt.Sprintf("d2vm-%s-root", uuid.New().String())
		// cryptsetup open -d $KEY_FILE $ROOT_DEVICE $ROOT_LABEL
		if err := exec.Run(ctx, "cryptsetup", "open", "--key-file", f.Name(), b.rootPart, b.cryptRoot); err != nil {
			return err
		}
		b.cryptPart = b.rootPart
		b.rootPart = "/dev/mapper/root"
		b.mappedCryptRoot = filepath.Join("/dev/mapper", b.cryptRoot)
		logrus.Infof("creating raw image file system")
		if err := exec.Run(ctx, "mkfs.ext4", b.mappedCryptRoot); err != nil {
			return err
		}
		if err := exec.Run(ctx, "mount", b.mappedCryptRoot, b.mntPoint); err != nil {
			return err
		}
	} else {
		logrus.Infof("creating raw image file system")
		if err := exec.Run(ctx, "mkfs.ext4", b.rootPart); err != nil {
			return err
		}
		if err := exec.Run(ctx, "mount", b.rootPart, b.mntPoint); err != nil {
			return err
		}
	}
	if !b.splitBoot {
		return nil
	}
	if err := os.MkdirAll(filepath.Join(b.mntPoint, "boot"), os.ModePerm); err != nil {
		return err
	}
	if err := exec.Run(ctx, "mkfs.ext4", b.bootPart); err != nil {
		return err
	}
	if err := exec.Run(ctx, "mount", b.bootPart, filepath.Join(b.mntPoint, "boot")); err != nil {
		return err
	}
	return nil
}

func (b *builder) unmountImg(ctx context.Context) error {
	logrus.Infof("unmounting raw image")
	var merr error
	if b.splitBoot {
		merr = multierr.Append(merr, exec.Run(ctx, "umount", filepath.Join(b.mntPoint, "boot")))
	}
	merr = multierr.Append(merr, exec.Run(ctx, "umount", b.mntPoint))
	if b.isLuksEnabled() {
		merr = multierr.Append(merr, exec.Run(ctx, "cryptsetup", "close", b.mappedCryptRoot))
	}
	return multierr.Combine(
		merr,
		exec.Run(ctx, "kpartx", "-d", b.loDevice),
		exec.Run(ctx, "losetup", "-d", b.loDevice),
	)
}

func (b *builder) copyRootFS(ctx context.Context) error {
	logrus.Infof("copying rootfs to raw image")
	if err := b.img.Flatten(ctx, b.mntPoint); err != nil {
		return err
	}
	return nil
}

func diskUUID(ctx context.Context, disk string) (string, error) {
	o, _, err := exec.RunOut(ctx, "blkid", "-s", "UUID", "-o", "value", disk)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(o, "\n"), nil
}

func (b *builder) setupRootFS(ctx context.Context) (err error) {
	logrus.Infof("setting up rootfs")
	b.rootUUID, err = diskUUID(ctx, b.rootPart)
	var fstab string
	if b.splitBoot {
		b.bootUUID, err = diskUUID(ctx, b.bootPart)
		if err != nil {
			return err
		}
		if b.isLuksEnabled() {
			b.cryptUUID, err = diskUUID(ctx, b.cryptPart)
			if err != nil {
				return err
			}
		}
		fstab = fmt.Sprintf("UUID=%s / ext4 errors=remount-ro 0 1\nUUID=%s /boot ext4 errors=remount-ro 0 2\n", b.rootUUID, b.bootUUID)
	} else {
		b.bootUUID = b.rootUUID
		fstab = fmt.Sprintf("UUID=%s / ext4 errors=remount-ro 0 1\n", b.bootUUID)
	}
	if err := b.chWriteFile("/etc/fstab", fstab, perm); err != nil {
		return err
	}
	if err := b.chWriteFileIfNotExist("/etc/resolv.conf", "nameserver 8.8.8.8", 0644); err != nil {
		return err
	}
	if err := b.chWriteFileIfNotExist("/etc/hostname", "localhost", perm); err != nil {
		return err
	}
	if err := b.chWriteFileIfNotExist("/etc/hosts", hosts, perm); err != nil {
		return err
	}
	// TODO(adphi): is it the righ fix ?
	if err := os.RemoveAll("/usr/sbin/policy-rc.d"); err != nil {
		return err
	}
	if err := os.RemoveAll(b.chPath("/.dockerenv")); err != nil {
		return err
	}
	// create a symlink to /boot for non-alpine images in order to have a consistent path
	// even if the image is not split
	if _, err := os.Stat(b.chPath("/boot/boot")); os.IsNotExist(err) {
		if err := os.Symlink(".", b.chPath("/boot/boot")); err != nil {
			return err
		}
	}
	switch b.osRelease.ID {
	case ReleaseAlpine:
		by, err := os.ReadFile(b.chPath("/etc/inittab"))
		if err != nil {
			return err
		}
		by = append(by, []byte("\n"+"ttyS0::respawn:/sbin/getty -L ttyS0 115200 vt100\n")...)
		if err := b.chWriteFile("/etc/inittab", string(by), perm); err != nil {
			return err
		}
		if err := b.chWriteFileIfNotExist("/etc/network/interfaces", "", perm); err != nil {
			return err
		}
		return nil
	case ReleaseUbuntu:
		if b.osRelease.VersionID >= "20.04" {
			return nil
		}
		fallthrough
	case ReleaseDebian, ReleaseKali:
		t, err := os.Readlink(b.chPath("/vmlinuz"))
		if err != nil {
			return err
		}
		if err := os.Symlink(t, b.chPath("/boot/vmlinuz")); err != nil {
			return err
		}
		t, err = os.Readlink(b.chPath("/initrd.img"))
		if err != nil {
			return err
		}
		if err := os.Symlink(t, b.chPath("/boot/initrd.img")); err != nil {
			return err
		}
		return nil
	default:
		return nil
	}
}

func (b *builder) installKernel(ctx context.Context) error {
	logrus.Infof("installing linux kernel")
	if err := exec.Run(ctx, "extlinux", "--install", b.chPath("/boot")); err != nil {
		return err
	}
	sysconfig, err := sysconfig(b.osRelease)
	if err != nil {
		return err
	}
	var cfg string
	if b.isLuksEnabled() {
		if b.osRelease.ID != ReleaseAlpine {
			cfg = fmt.Sprintf(sysconfig, b.rootUUID, fmt.Sprintf("%s root=/dev/mapper/root cryptopts=target=root,source=UUID=%s", b.cmdLineExtra, b.cryptUUID))
			cfg = strings.Replace(cfg, "root=UUID="+b.rootUUID, "", 1)
		} else {
			cfg = fmt.Sprintf(sysconfig, b.rootUUID, fmt.Sprintf("%s root=/dev/mapper/root cryptdm=root", b.cmdLineExtra))
			cfg = strings.Replace(cfg, "root=UUID="+b.rootUUID, "cryptroot=UUID="+b.cryptUUID, 1)
		}
	} else {
		cfg = fmt.Sprintf(sysconfig, b.rootUUID, b.cmdLineExtra)
	}
	if err := b.chWriteFile("/boot/syslinux.cfg", cfg, perm); err != nil {
		return err
	}
	return nil
}

func (b *builder) setupMBR(ctx context.Context) error {
	logrus.Infof("writing MBR")
	if err := exec.Run(ctx, "dd", fmt.Sprintf("if=%s", b.mbrPath), fmt.Sprintf("of=%s", b.diskRaw), "bs=440", "count=1", "conv=notrunc"); err != nil {
		return err
	}
	return nil
}

func (b *builder) convert2Img(ctx context.Context) error {
	logrus.Infof("converting to %s", b.format)
	return exec.Run(ctx, "qemu-img", "convert", b.diskRaw, "-O", b.format, b.diskOut)
}

func (b *builder) chWriteFile(path string, content string, perm os.FileMode) error {
	return os.WriteFile(b.chPath(path), []byte(content), perm)
}

func (b *builder) chWriteFileIfNotExist(path string, content string, perm os.FileMode) error {
	if i, err := os.Stat(b.chPath(path)); err == nil && i.Size() != 0 {
		return nil
	}
	return os.WriteFile(b.chPath(path), []byte(content), perm)
}

func (b *builder) chPath(path string) string {
	return fmt.Sprintf("%s%s", b.mntPoint, path)
}

func (b *builder) isLuksEnabled() bool {
	return b.luksPassword != ""
}

func (b *builder) Close() error {
	return b.img.Close()
}

func block(path string, size uint64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Truncate(int64(size))
}

func checkDependencies() error {
	var merr error
	for _, v := range []string{"mount", "blkid", "tar", "losetup", "parted", "kpartx", "qemu-img", "extlinux", "dd", "mkfs.ext4", "cryptsetup"} {
		if _, err := exec2.LookPath(v); err != nil {
			merr = multierr.Append(merr, err)
		}
	}
	return merr
}

func OutputFormats() []string {
	return formats[:]
}
