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

	"github.com/sirupsen/logrus"
)

type grubBios struct {
	*grubCommon
}

func (g grubBios) Validate(_ BootFS) error {
	return nil
}

func (g grubBios) Setup(ctx context.Context, dev, root string, cmdline string) error {
	logrus.Infof("setting up grub bootloader")
	clean, err := g.prepare(ctx, dev, root, cmdline)
	if err != nil {
		return err
	}
	defer clean()
	if err := g.install(ctx, "--target=i386-pc", "--boot-directory=/boot", dev); err != nil {
		return err
	}
	if err := g.mkconfig(ctx); err != nil {
		return err
	}
	return nil
}

type grubBiosProvider struct {
	config Config
}

func (g grubBiosProvider) New(c Config, r OSRelease) (Bootloader, error) {
	return grubBios{grubCommon: newGrubCommon(c, r)}, nil
}

func (g grubBiosProvider) Name() string {
	return "grub-bios"
}

func init() {
	RegisterBootloaderProvider(grubBiosProvider{})
}
