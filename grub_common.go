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

const grubCfg = `GRUB_DEFAULT=0
GRUB_HIDDEN_TIMEOUT=0
GRUB_HIDDEN_TIMEOUT_QUIET=true
GRUB_TIMEOUT=0
GRUB_CMDLINE_LINUX_DEFAULT="%s"
GRUB_CMDLINE_LINUX=""
GRUB_TERMINAL=console
`

type grubCommon struct {
	name string
	c    Config
	r    OSRelease
	root string
	dev  string
}

func newGrubCommon(c Config, r OSRelease) *grubCommon {
	name := "grub"
	if r.ID == "centos" {
		name = "grub2"
	}
	return &grubCommon{
		name: name,
		c:    c,
		r:    r,
	}
}

func (g *grubCommon) prepare(ctx context.Context, dev, root, cmdline string) (clean func(), err error) {
	g.dev = dev
	g.root = root
	if err = os.WriteFile(filepath.Join(root, "etc", "default", "grub"), []byte(fmt.Sprintf(grubCfg, cmdline)), perm); err != nil {
		return
	}
	if err = os.MkdirAll(filepath.Join(root, "boot", g.name), os.ModePerm); err != nil {
		return
	}
	mounts := []string{"dev", "proc", "sys"}
	var unmounts []string
	clean = func() {
		for _, v := range unmounts {
			if err := exec.Run(ctx, "umount", filepath.Join(root, v)); err != nil {
				logrus.Errorf("failed to unmount /%s: %s", v, err)
			}
		}
	}
	defer func() {
		if err != nil {
			clean()
		}
	}()
	for _, v := range mounts {
		if err = exec.Run(ctx, "mount", "-o", "bind", "/"+v, filepath.Join(root, v)); err != nil {
			return
		}
		unmounts = append(unmounts, v)
	}
	return
}

func (g *grubCommon) install(ctx context.Context, args ...string) error {
	if g.dev == "" || g.root == "" {
		return fmt.Errorf("grubCommon not prepared")
	}
	args = append([]string{g.root, g.name + "-install"}, args...)
	return exec.Run(ctx, "chroot", args...)
}

func (g *grubCommon) mkconfig(ctx context.Context) error {
	if g.dev == "" || g.root == "" {
		return fmt.Errorf("grubCommon not prepared")
	}
	return exec.Run(ctx, "chroot", g.root, g.name+"-mkconfig", "-o", "/boot/"+g.name+"/grub.cfg")
}
