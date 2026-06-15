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
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

type grubEFI struct {
	*grubCommon
	arch string
}

func (g grubEFI) Validate(fs BootFS) error {
	switch fs {
	case BootFSFat32:
		return nil
	default:
		return fmt.Errorf("grub-efi only supports fat32 boot filesystem")
	}
}

func (g grubEFI) Setup(ctx context.Context, dev, root string, cmdline string) error {
	logrus.Infof("setting up grub-efi bootloader")
	clean, err := g.prepare(ctx, dev, root, cmdline)
	if err != nil {
		return err
	}
	defer clean()
	if err := g.install(ctx, "--target="+g.arch+"-efi", "--efi-directory=/boot", "--no-nvram", "--removable", "--no-floppy", "--force"); err != nil {
		return err
	}
	if err := g.mkconfig(ctx); err != nil {
		return err
	}
	return nil
}

type grubEFIProvider struct{}

func (g grubEFIProvider) New(_ Config, r OSRelease, arch string) (Bootloader, error) {
	if err := checkGrubEFISupport(r); err != nil {
		return nil, err
	}
	return grubEFI{grubCommon: newGrubCommon(r), arch: arch}, nil
}

func (g grubEFIProvider) Name() string {
	return "grub-efi"
}

func checkGrubEFISupport(r OSRelease) error {
	if r.ID == ReleaseCentOS {
		return fmt.Errorf("grub (efi) is not supported for CentOS, use grub-bios instead")
	}
	v, _ := strconv.Atoi(strings.Split(r.VersionID, ".")[0])
	if (r.ID == ReleaseAlmaLinux || r.ID == ReleaseRocky) && v < 9 {
		return fmt.Errorf("grub (efi) is not supported for %s (%s), use 9+ releases or grub-bios instead", r.ID, r.VersionID)
	}
	return nil
}

func init() {
	RegisterBootloaderProvider(grubEFIProvider{})
}
