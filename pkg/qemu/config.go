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

package qemu

import (
	"io"

	"github.com/google/uuid"
)

type Option func(c *config)

type Disk struct {
	Path   string
	Size   int
	Format string
}

type PublishedPort struct {
	Guest    uint16
	Host     uint16
	Protocol string
}

// config contains the config for Qemu
type config struct {
	path           string
	uuid           uuid.UUID
	gui            bool
	disks          []Disk
	networking     string
	arch           string
	cpus           uint
	memory         uint
	bios           string
	accel          string
	detached       bool
	qemuBinPath    string
	qemuImgPath    string
	publishedPorts []PublishedPort
	netdevConfig   string

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func WithGUI() Option {
	return func(c *config) {
		c.gui = true
	}
}

func WithDisks(disks ...Disk) Option {
	return func(c *config) {
		c.disks = disks
	}
}

func WithNetworking(networking string) Option {
	return func(c *config) {
		c.networking = networking
	}
}

func WithArch(arch string) Option {
	return func(c *config) {
		c.arch = arch
	}
}

func WithCPUs(cpus uint) Option {
	return func(c *config) {
		c.cpus = cpus
	}
}

func WithMemory(memory uint) Option {
	return func(c *config) {
		c.memory = memory
	}
}

func WithBios(bios string) Option {
	return func(c *config) {
		c.bios = bios
	}
}

func WithAccel(accel string) Option {
	return func(c *config) {
		c.accel = accel
	}
}

func WithDetached() Option {
	return func(c *config) {
		c.detached = true
	}
}

func WithQemuBinPath(path string) Option {
	return func(c *config) {
		c.qemuBinPath = path
	}
}

func WithQemuImgPath(path string) Option {
	return func(c *config) {
		c.qemuImgPath = path
	}
}

func WithPublishedPorts(ports ...PublishedPort) Option {
	return func(c *config) {
		c.publishedPorts = ports
	}
}

func WithStdin(r io.Reader) Option {
	return func(c *config) {
		c.stdin = r
	}
}

func WithStdout(w io.Writer) Option {
	return func(c *config) {
		c.stdout = w
	}
}

func WithStderr(w io.Writer) Option {
	return func(c *config) {
		c.stderr = w
	}
}
