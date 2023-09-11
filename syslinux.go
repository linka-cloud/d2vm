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
	"path/filepath"

	"github.com/sirupsen/logrus"

	"go.linka.cloud/d2vm/pkg/exec"
)

const syslinuxCfg = `DEFAULT linux
  SAY Now booting the kernel from SYSLINUX...
 LABEL linux
  KERNEL %s
  APPEND %s
`

var mbrPaths = []string{
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

type syslinux struct {
	c      Config
	mbrBin string
}

func (s syslinux) Setup(ctx context.Context, dev, root string, cmdline string) error {
	logrus.Infof("setting up syslinux bootloader")
	if err := exec.Run(ctx, "extlinux", "--install", filepath.Join(root, "boot")); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "boot", "syslinux.cfg"), []byte(fmt.Sprintf(syslinuxCfg, s.c.Kernel, cmdline)), perm); err != nil {
		return err
	}
	logrus.Infof("writing MBR")
	if err := exec.Run(ctx, "dd", fmt.Sprintf("if=%s", s.mbrBin), fmt.Sprintf("of=%s", dev), "bs=440", "count=1", "conv=notrunc"); err != nil {
		return err
	}
	return nil
}

type syslinuxProvider struct{}

func (s syslinuxProvider) New(c Config, _ OSRelease) (Bootloader, error) {
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
	return &syslinux{
		c:      c,
		mbrBin: mbrBin,
	}, nil
}

func (s syslinuxProvider) Name() string {
	return "syslinux"
}

func init() {
	RegisterBootloaderProvider(syslinuxProvider{})
}
