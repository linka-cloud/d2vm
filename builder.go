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
  APPEND ro root=UUID=%s initrd=/boot/initrd.img net.ifnames=0 console=tty0 console=ttyS0,115200n8
`
	syslinuxCfgDebian = `DEFAULT linux
  SAY Now booting the kernel from SYSLINUX...
 LABEL linux
  KERNEL /vmlinuz
  APPEND ro root=UUID=%s initrd=/initrd.img net.ifnames=0 console=tty0 console=ttyS0,115200n8
`
	syslinuxCfgAlpine = `DEFAULT linux
  SAY Now booting the kernel from SYSLINUX...
 LABEL linux
  KERNEL /boot/vmlinuz-virt
  APPEND ro root=UUID=%s rootfstype=ext4 initrd=/boot/initramfs-virt console=ttyS0,115200
`
	syslinuxCfgCentOS = `DEFAULT linux
  SAY Now booting the kernel from SYSLINUX...
 LABEL linux
  KERNEL /boot/vmlinuz
  APPEND ro root=UUID=%s initrd=/boot/initrd.img net.ifnames=0 console=tty0 console=ttyS0,115200n8
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
	case ReleaseAlpine:
		return syslinuxCfgAlpine, nil
	case ReleaseCentOS:
		return syslinuxCfgCentOS, nil
	default:
		return "", fmt.Errorf("%s: distribution not supported", osRelease.ID)
	}
}

type builder struct {
	osRelease OSRelease

	src     string
	img     *image
	diskRaw string
	diskOut string
	format  string

	size     int64
	mntPoint string

	mbrPath string

	loDevice string
	loPart   string
	diskUUD  string
}

func NewBuilder(ctx context.Context, workdir, imgTag, disk string, size int64, osRelease OSRelease, format string) (*builder, error) {
	if err := checkDependencies(); err != nil {
		return nil, err
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
		size = 10 * int64(datasize.GB)
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
		osRelease: osRelease,
		img:       img,
		diskRaw:   filepath.Join(workdir, disk+".d2vm.raw"),
		diskOut:   filepath.Join(workdir, disk+"."+format),
		format:    f,
		size:      size,
		mbrPath:   mbrBin,
		mntPoint:  filepath.Join(workdir, "/mnt"),
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

	if err := exec.Run(ctx, "parted", "-s", b.diskRaw, "mklabel", "msdos", "mkpart", "primary", "1Mib", "100%", "set", "1", "boot", "on"); err != nil {
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
	if err := exec.Run(ctx, "partprobe", b.loDevice); err != nil {
		return err
	}
	b.loPart = fmt.Sprintf("%sp1", b.loDevice)
	logrus.Infof("creating raw image file system")
	if err := exec.Run(ctx, "mkfs.ext4", b.loPart); err != nil {
		return err
	}
	if err := exec.Run(ctx, "mount", b.loPart, b.mntPoint); err != nil {
		return err
	}
	return nil
}

func (b *builder) unmountImg(ctx context.Context) error {
	logrus.Infof("unmounting raw image")
	var merr error
	if err := exec.Run(ctx, "umount", b.mntPoint); err != nil {
		merr = multierr.Append(merr, err)
	}
	if err := exec.Run(ctx, "losetup", "-d", b.loDevice); err != nil {
		merr = multierr.Append(merr, err)
	}
	return merr
}

func (b *builder) copyRootFS(ctx context.Context) error {
	logrus.Infof("copying rootfs to raw image")
	if err := b.img.Flatten(ctx, b.mntPoint); err != nil {
		return err
	}
	return nil
}

func (b *builder) setupRootFS(ctx context.Context) error {
	logrus.Infof("setting up rootfs")
	o, _, err := exec.RunOut(ctx, "blkid", "-s", "UUID", "-o", "value", b.loPart)
	if err != nil {
		return err
	}
	b.diskUUD = strings.TrimSuffix(o, "\n")
	fstab := fmt.Sprintf("UUID=%s / ext4 errors=remount-ro 0 1\n", b.diskUUD)
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
	if err := os.RemoveAll("/ur/sbin/policy-rc.d"); err != nil {
		return err
	}
	if err := os.RemoveAll(b.chPath("/.dockerenv")); err != nil {
		return err
	}
	if b.osRelease.ID != ReleaseAlpine {
		return nil
	}
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
	if err := b.chWriteFile("/boot/syslinux.cfg", fmt.Sprintf(sysconfig, b.diskUUD), perm); err != nil {
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

func (b *builder) Close() error {
	return b.img.Close()
}

func block(path string, size int64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Truncate(size)
}

func checkDependencies() error {
	var merr error
	for _, v := range []string{"mount", "blkid", "tar", "losetup", "parted", "partprobe", "qemu-img", "extlinux", "dd", "mkfs"} {
		if _, err := exec2.LookPath(v); err != nil {
			merr = multierr.Append(merr, err)
		}
	}
	return merr
}

func OutputFormats() []string {
	return formats[:]
}
