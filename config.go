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
	"fmt"
	"strings"
)

var (
	configUbuntu = Config{
		Kernel: "/boot/vmlinuz",
		Initrd: "/boot/initrd.img",
	}
	configDebian = Config{
		Kernel: "/boot/vmlinuz",
		Initrd: "/boot/initrd.img",
	}
	configAlpine = Config{
		Kernel: "/boot/vmlinuz-virt",
		Initrd: "/boot/initramfs-virt",
	}
	configCentOS = Config{
		Kernel: "/boot/vmlinuz",
		Initrd: "/boot/initrd.img",
	}
)

type Root interface {
	String() string
}

type RootUUID string

func (r RootUUID) String() string {
	return "UUID=" + string(r)
}

type RootPath string

func (r RootPath) String() string {
	return string(r)
}

type Config struct {
	Kernel string
	Initrd string
}

func (c Config) Cmdline(root Root, rootFS RootFS, args ...string) string {
	var r string
	if root != nil {
		r = fmt.Sprintf("root=%s", root.String())
	}
	return fmt.Sprintf("ro initrd=%s %s net.ifnames=0 rootfstype=%s console=tty0 console=ttyS0,115200n8 %s", c.Initrd, r, rootFS, strings.Join(args, " "))
}

func (r OSRelease) Config() (Config, error) {
	switch r.ID {
	case ReleaseUbuntu:
		if r.VersionID < "20.04" {
			return configDebian, nil
		}
		return configUbuntu, nil
	case ReleaseDebian:
		return configDebian, nil
	case ReleaseKali:
		return configDebian, nil
	case ReleaseAlpine:
		return configAlpine, nil
	case ReleaseCentOS:
		return configCentOS, nil
	default:
		return Config{}, fmt.Errorf("%s: distribution not supported", r.ID)

	}
}
