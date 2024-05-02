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
	perm os.FileMode = 0644
)

var formats = []string{"qcow2", "qed", "raw", "vdi", "vhdx", "vhd", "vmdk"}

type Builder interface {
	Build(ctx context.Context) (err error)
	Close() error
}

type builder struct {
	osRelease  OSRelease
	config     Config
	bootloader Bootloader

	src     string
	img     *image
	diskRaw string
	diskOut string
	format  string

	size     uint64
	mntPoint string

	splitBoot bool
	bootSize  uint64
	bootFS    BootFS

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
	arch         string
}

func NewBuilder(ctx context.Context, workdir, imgTag, disk string, size uint64, osRelease OSRelease, format string, cmdLineExtra string, splitBoot bool, bootFS BootFS, bootSize uint64, luksPassword string, bootLoader string, platform string) (Builder, error) {
	var arch string
	switch platform {
	case "linux/amd64":
		arch = "x86_64"
	case "linux/arm64", "linux/aarch64":
		arch = "arm64"
	default:
		return nil, fmt.Errorf("unexpected platform: %s, supported platforms: linux/amd64, linux/arm64", platform)
	}
	if luksPassword != "" {
		if !splitBoot {
			return nil, fmt.Errorf("luks encryption requires split boot")
		}
		if !osRelease.SupportsLUKS() {
			return nil, fmt.Errorf("luks encryption not supported on %s %s", osRelease.ID, osRelease.VersionID)
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

	if bootLoader == "" {
		bootLoader = "syslinux"
	}

	config, err := osRelease.Config()
	if err != nil {
		return nil, err
	}

	if splitBoot {
		config.Kernel = strings.TrimPrefix(config.Kernel, "/boot")
		config.Initrd = strings.TrimPrefix(config.Initrd, "/boot")
	}

	if bootFS == "" {
		bootFS = BootFSExt4
	}

	if err := bootFS.Validate(); err != nil {
		return nil, err
	}

	blp, err := BootloaderByName(bootLoader)
	if err != nil {
		return nil, err
	}
	bl, err := blp.New(config, osRelease, arch)
	if err != nil {
		return nil, err
	}

	if err := bl.Validate(bootFS); err != nil {
		return nil, err
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
		config:       config,
		bootloader:   bl,
		img:          img,
		diskRaw:      filepath.Join(workdir, disk+".d2vm.raw"),
		diskOut:      filepath.Join(workdir, disk+"."+format),
		format:       f,
		size:         size,
		mntPoint:     filepath.Join(workdir, "/mnt"),
		cmdLineExtra: cmdLineExtra,
		splitBoot:    splitBoot,
		bootSize:     bootSize,
		bootFS:       bootFS,
		luksPassword: luksPassword,
		arch:         arch,
	}
	if err := b.checkDependencies(); err != nil {
		return nil, err
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
	if err = b.installBootloader(ctx); err != nil {
		return err
	}
	if err = b.unmountImg(ctx); err != nil {
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
	b.rootPart = ifElse(b.splitBoot, fmt.Sprintf("/dev/mapper/%sp2", filepath.Base(b.loDevice)), b.bootPart)
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
	if b.bootFS.IsFat() {
		err = exec.Run(ctx, "mkfs.fat", "-F32", b.bootPart)
	} else {
		err = exec.Run(ctx, "mkfs.ext4", b.bootPart)
	}
	if err != nil {
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
	b.rootUUID, err = diskUUID(ctx, ifElse(b.isLuksEnabled(), b.mappedCryptRoot, b.rootPart))
	if err != nil {
		return err
	}
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
		fstab = fmt.Sprintf("UUID=%s / ext4 errors=remount-ro 0 1\nUUID=%s /boot %s errors=remount-ro 0 2\n", b.rootUUID, b.bootUUID, b.bootFS.linux())
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
	default:
		return nil
	}
}

func (b *builder) cmdline(_ context.Context) string {
	if !b.isLuksEnabled() {
		return b.config.Cmdline(RootUUID(b.rootUUID), b.cmdLineExtra)
	}
	switch b.osRelease.ID {
	case ReleaseAlpine:
		return b.config.Cmdline(RootUUID(b.rootUUID), "root=/dev/mapper/root", "cryptdm=root", "cryptroot=UUID="+b.cryptUUID, b.cmdLineExtra)
	case ReleaseCentOS:
		return b.config.Cmdline(RootUUID(b.rootUUID), "rd.luks.name=UUID="+b.rootUUID+" rd.luks.uuid="+b.cryptUUID+" rd.luks.crypttab=0", b.cmdLineExtra)
	default:
		// for some versions of debian, the cryptopts parameter MUST contain all the following: target,source,key,opts...
		// see https://salsa.debian.org/cryptsetup-team/cryptsetup/-/blob/debian/buster/debian/functions
		// and https://cryptsetup-team.pages.debian.net/cryptsetup/README.initramfs.html
		return b.config.Cmdline(nil, "root=/dev/mapper/root", "cryptopts=target=root,source=UUID="+b.cryptUUID+",key=none,luks", b.cmdLineExtra)
	}
}

func (b *builder) installBootloader(ctx context.Context) error {
	logrus.Infof("installing bootloader")
	return b.bootloader.Setup(ctx, b.loDevice, b.mntPoint, b.cmdline(ctx))
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

func (b *builder) checkDependencies() error {
	var merr error
	deps := []string{"mount", "blkid", "tar", "losetup", "parted", "kpartx", "qemu-img", "dd", "mkfs.ext4", "cryptsetup"}
	if _, ok := b.bootloader.(*syslinux); ok {
		deps = append(deps, "extlinux")
	}
	for _, v := range deps {
		if _, err := exec2.LookPath(v); err != nil {
			merr = multierr.Append(merr, err)
		}
	}
	return merr
}

func OutputFormats() []string {
	return formats[:]
}

func ifElse(v bool, t string, f string) string {
	if v {
		return t
	}
	return f
}
