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

package docker2vm

import (
	"bytes"
	"context"
	"fmt"
	"os"
	exec2 "os/exec"
	"path/filepath"
	"strings"

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
	mbrBin = "/usr/lib/EXTLINUX/mbr.bin"
)

var (
	fdiskCmds = []string{"n", "p", "1", "", "", "a", "w"}
)

type builder struct {
	osRelease OSRelease

	src       string
	diskRaw   string
	diskQcow2 string
	size      int64
	mntPoint  string

	loDevice string
	loPart   string
	diskUUD  string
}

func NewBuilder(workdir, src, disk string, size int64, osRelease OSRelease) (*builder, error) {
	if err := checkDependencies(); err != nil {
		return nil, err
	}
	if size == 0 {
		size = 1
	}
	if disk == "" {
		disk = "disk0"
	}
	b := &builder{
		osRelease: osRelease,
		src:       src,
		diskRaw:   filepath.Join(workdir, disk+".raw"),
		diskQcow2: filepath.Join(workdir, disk+".qcow2"),
		size:      size,
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
	if err = b.convert2Qcow2(ctx); err != nil {
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
	c := exec.CommandContext(ctx, "fdisk", b.diskRaw)
	var i bytes.Buffer
	for _, v := range fdiskCmds {
		if _, err := i.Write([]byte(v + "\n")); err != nil {
			return err
		}
	}
	var e bytes.Buffer
	c.Stdin = &i
	c.Stderr = &e
	if err := c.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, e.String())
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
	b.loPart = fmt.Sprintf("/dev/mapper/%sp1", filepath.Base(b.loDevice))
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
	if err := exec.Run(ctx, "kpartx", "-d", b.loDevice); err != nil {
		merr = multierr.Append(merr, err)
	}
	if err := exec.Run(ctx, "losetup", "-d", b.loDevice); err != nil {
		merr = multierr.Append(merr, err)
	}
	return merr
}

func (b *builder) copyRootFS(ctx context.Context) error {
	logrus.Infof("copying rootfs to raw image")
	if err := exec.Run(ctx, "tar", "-xvf", b.src, "-C", b.mntPoint); err != nil {
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
	if err := b.chWriteFile("/etc/fstab", fstab, 0644); err != nil {
		return err
	}
	if err := b.chWriteFile("/etc/resolv.conf", "nameserver 8.8.8.8", 0644); err != nil {
		return err
	}
	if err := b.chWriteFile("/etc/hostname", "localhost", 0644); err != nil {
		return err
	}
	if err := b.chWriteFile("/etc/hosts", hosts, 0644); err != nil {
		return err
	}
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
	if err := b.chWriteFile("/etc/inittab", string(by), 0644); err != nil {
		return err
	}
	if err := b.chWriteFile("/etc/network/interfaces", "", 0644); err != nil {
		return err
	}
	return nil
}

func (b *builder) installKernel(ctx context.Context) error {
	logrus.Infof("installing linux kernel")
	if err := exec.Run(ctx, "extlinux", "--install", b.chPath("/boot")); err != nil {
		return err
	}
	var sysconfig string
	switch b.osRelease.ID {
	case ReleaseUbuntu:
		sysconfig = syslinuxCfgUbuntu
	case ReleaseDebian:
		sysconfig = syslinuxCfgDebian
	case ReleaseAlpine:
		sysconfig = syslinuxCfgAlpine
	case ReleaseCentOS:
		sysconfig = syslinuxCfgCentOS
	default:
		return fmt.Errorf("%s: distribution not supported", b.osRelease.ID)
	}
	if err := b.chWriteFile("/boot/syslinux.cfg", fmt.Sprintf(sysconfig, b.diskUUD), 0644); err != nil {
		return err
	}
	return nil
}

func (b *builder) setupMBR(ctx context.Context) error {
	logrus.Infof("writing MBR")
	if err := exec.Run(ctx, "dd", fmt.Sprintf("if=%s", mbrBin), fmt.Sprintf("of=%s", b.diskRaw), "bs=440", "count=1", "conv=notrunc"); err != nil {
		return err
	}
	return nil
}

func (b *builder) convert2Qcow2(ctx context.Context) error {
	logrus.Infof("converting to QCOW2")
	return exec.Run(ctx, "qemu-img", "convert", b.diskRaw, "-O", "qcow2", b.diskQcow2)
}

func (b *builder) chWriteFile(path string, content string, perm os.FileMode) error {
	return os.WriteFile(b.chPath(path), []byte(content), perm)
}

func (b *builder) chPath(path string) string {
	return fmt.Sprintf("%s%s", b.mntPoint, path)
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
	for _, v := range []string{"mount", "blkid", "tar", "kpartx", "losetup", "qemu-img", "extlinux", "dd", "mkfs", "fdisk"} {
		if _, err := exec2.LookPath(v); err != nil {
			merr = multierr.Append(merr, err)
		}
	}
	if _, err := os.Stat(mbrBin); err != nil {
		merr = multierr.Append(merr, err)
	}
	return merr
}
