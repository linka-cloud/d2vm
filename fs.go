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
)

type BootFS string
type RootFS string

const (
	BootFSExt4  BootFS = "ext4"
	BootFSFat32 BootFS = "fat32"
)

const (
	RootFSExt4  RootFS = "ext4"
	RootFSBtrfs RootFS = "btrfs"
)

func (f BootFS) String() string {
	return string(f)
}

func (f BootFS) IsExt() bool {
	return f == BootFSExt4
}

func (f BootFS) IsFat() bool {
	return f == BootFSFat32
}

func (f BootFS) IsSupported() bool {
	return f.IsExt() || f.IsFat()
}

func (f BootFS) Validate() error {
	if !f.IsSupported() {
		return fmt.Errorf("invalid boot filesystem: %s valid filesystems are: fat32, ext4", f)
	}
	return nil
}

func (f BootFS) linux() string {
	switch f {
	case BootFSFat32:
		return "vfat"
	default:
		return "ext4"
	}
}
