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

	"github.com/sirupsen/logrus"
)

type grub struct {
	*grubCommon
}

func (g grub) Validate(fs BootFS) error {
	switch fs {
	case BootFSFat32:
		return nil
	default:
		return fmt.Errorf("grub only supports fat32 boot filesystem due to grub-efi")
	}
}

func (g grub) Setup(ctx context.Context, dev, root string, cmdline string) error {
	logrus.Infof("setting up grub bootloader")
	clean, err := g.prepare(ctx, dev, root, cmdline)
	if err != nil {
		return err
	}
	defer clean()
	if err := g.install(ctx, "--target=x86_64-efi", "--efi-directory=/boot", "--no-nvram", "--removable", "--no-floppy"); err != nil {
		return err
	}
	if err := g.install(ctx, "--target=i386-pc", "--boot-directory=/boot", dev); err != nil {
		return err
	}
	if err := g.mkconfig(ctx); err != nil {
		return err
	}
	return nil
}

type grubProvider struct {
	config Config
}

func (g grubProvider) New(c Config, r OSRelease, arch string) (Bootloader, error) {
	if arch != "x86_64" {
		return nil, fmt.Errorf("grub is only supported for amd64")
	}
	if r.ID == ReleaseCentOS || r.ID == ReleaseRocky || r.ID == ReleaseAlmaLinux {
		return nil, fmt.Errorf("grub (efi) is not supported for CentOS / Rocky / AlmaLinux, use grub-bios instead")
	}
	return grub{grubCommon: newGrubCommon(c, r)}, nil
}

func (g grubProvider) Name() string {
	return "grub"
}

func init() {
	RegisterBootloaderProvider(grubProvider{})
}
