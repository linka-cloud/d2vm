// Copyright 2023 Linka Cloud  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
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

	"github.com/sirupsen/logrus"

	"go.linka.cloud/d2vm/pkg/exec"
)

const grubCfg = `GRUB_DEFAULT=0
GRUB_HIDDEN_TIMEOUT=0
GRUB_HIDDEN_TIMEOUT_QUIET=true
GRUB_TIMEOUT=0
GRUB_CMDLINE_LINUX_DEFAULT="%s"
GRUB_CMDLINE_LINUX=""
GRUB_TERMINAL=console
`

type grub struct {
	name string
	c    Config
	r    OSRelease
}

func (g grub) Setup(ctx context.Context, dev, root string, cmdline string) error {
	logrus.Infof("setting up grub bootloader")
	if err := os.WriteFile(filepath.Join(root, "etc", "default", "grub"), []byte(fmt.Sprintf(grubCfg, cmdline)), perm); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, "boot", g.name), os.ModePerm); err != nil {
		return err
	}
	mounts := []string{"dev", "proc", "sys"}
	var unmounts []string
	defer func() {
		for _, v := range unmounts {
			if err := exec.Run(ctx, "umount", filepath.Join(root, v)); err != nil {
				logrus.Errorf("failed to unmount /%s: %s", v, err)
			}
		}
	}()
	for _, v := range mounts {
		if err := exec.Run(ctx, "mount", "-o", "bind", "/"+v, filepath.Join(root, v)); err != nil {
			return err
		}
		unmounts = append(unmounts, v)
	}

	if err := exec.Run(ctx, "chroot", root, g.name+"-install", "--target=i386-pc", "--boot-directory", "/boot", dev); err != nil {
		return err
	}
	if err := exec.Run(ctx, "chroot", root, g.name+"-mkconfig", "-o", "/boot/"+g.name+"/grub.cfg"); err != nil {
		return err
	}
	return nil
}

type grubBootloaderProvider struct {
	config Config
}

func (g grubBootloaderProvider) New(c Config, r OSRelease) (Bootloader, error) {
	name := "grub"
	if r.ID == "centos" {
		name = "grub2"
	}
	if _, err := exec2.LookPath("grub-install"); err != nil {
		return nil, err
	}
	if _, err := exec2.LookPath("grub-mkconfig"); err != nil {
		return nil, err
	}
	return grub{
		name: name,
		c:    c,
		r:    r,
	}, nil
}

func (g grubBootloaderProvider) Name() string {
	return "grub"
}

func init() {
	RegisterBootloaderProvider(grubBootloaderProvider{})
}
