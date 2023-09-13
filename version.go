// Copyright 2022 Linka Cloud  All rights reserved.
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
	"runtime"

	"go.linka.cloud/d2vm/pkg/qemu_img"
)

var (
	Arch      = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	Version   = ""
	BuildDate = ""
	Image     = "linkacloud/d2vm"
)

func init() {
	qemu_img.DockerImageName = Image
	qemu_img.DockerImageVersion = Version
}
