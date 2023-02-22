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

package main

import (
	"strings"

	"github.com/spf13/pflag"

	"go.linka.cloud/d2vm"
)

var (
	output           = "disk0.qcow2"
	size             = "1G"
	password         = ""
	force            = false
	raw              bool
	pull             = false
	cmdLineExtra     = ""
	containerDiskTag = ""
	push             bool
	networkManager   string
	splitBoot        bool
	bootSize         uint64
)

func buildFlags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("build", pflag.ExitOnError)
	flags.StringVarP(&output, "output", "o", output, "The output image, the extension determine the image format, raw will be used if none. Supported formats: "+strings.Join(d2vm.OutputFormats(), " "))
	flags.StringVarP(&password, "password", "p", "", "Optional root user password")
	flags.StringVarP(&size, "size", "s", "10G", "The output image size")
	flags.BoolVar(&force, "force", false, "Override output qcow2 image")
	flags.StringVar(&cmdLineExtra, "append-to-cmdline", "", "Extra kernel cmdline arguments to append to the generated one")
	flags.StringVar(&networkManager, "network-manager", "", "Network manager to use for the image: none, netplan, ifupdown")
	flags.BoolVar(&raw, "raw", false, "Just convert the container to virtual machine image without installing anything more")
	flags.StringVarP(&containerDiskTag, "tag", "t", "", "Container disk Docker image tag")
	flags.BoolVar(&push, "push", false, "Push the container disk image to the registry")
	flags.BoolVar(&splitBoot, "split-boot", false, "Split the boot partition from the root partition")
	flags.Uint64Var(&bootSize, "boot-size", 100, "Size of the boot partition in MB")
	return flags
}
