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
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.linka.cloud/d2vm/pkg/docker"
	"go.linka.cloud/d2vm/pkg/exec"
)

func testSysconfig(t *testing.T, ctx context.Context, img, sysconf, kernel, initrd string) {
	require.NoError(t, docker.Pull(ctx, img))
	tmpPath := filepath.Join(os.TempDir(), "d2vm-tests", strings.NewReplacer(":", "-", ".", "-").Replace(img))
	require.NoError(t, os.MkdirAll(tmpPath, 0755))
	defer os.RemoveAll(tmpPath)
	logrus.Infof("inspecting image %s", img)
	r, err := FetchDockerImageOSRelease(ctx, img, tmpPath)
	require.NoError(t, err)
	defer docker.Remove(ctx, img)
	sys, err := sysconfig(r)
	require.NoError(t, err)
	assert.Equal(t, sysconf, sys)
	d, err := NewDockerfile(r, img, "root", "")
	require.NoError(t, err)
	logrus.Infof("docker image based on %s", d.Release.Name)
	p := filepath.Join(tmpPath, docker.FormatImgName(img))
	dir := filepath.Dir(p)
	f, err := os.Create(p)
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, d.Render(f))
	imgUUID := uuid.New().String()
	logrus.Infof("building kernel enabled image")
	require.NoError(t, docker.Build(ctx, imgUUID, p, dir))
	defer docker.Remove(ctx, imgUUID)
	require.NoError(t, docker.RunAndRemove(ctx, imgUUID, "test", "-f", kernel))
	require.NoError(t, docker.RunAndRemove(ctx, imgUUID, "test", "-f", initrd))
}

func TestSyslinuxCfg(t *testing.T) {
	t.Parallel()
	tests := []struct {
		image     string
		kernel    string
		initrd    string
		sysconfig string
	}{
		{
			image:     "ubuntu:18.04",
			kernel:    "/vmlinuz",
			initrd:    "/initrd.img",
			sysconfig: syslinuxCfgDebian,
		},
		{
			image:     "ubuntu:20.04",
			kernel:    "/boot/vmlinuz",
			initrd:    "/boot/initrd.img",
			sysconfig: syslinuxCfgUbuntu,
		},
		{
			image:     "ubuntu:22.04",
			kernel:    "/boot/vmlinuz",
			initrd:    "/boot/initrd.img",
			sysconfig: syslinuxCfgUbuntu,
		},
		{
			image:     "ubuntu:latest",
			kernel:    "/boot/vmlinuz",
			initrd:    "/boot/initrd.img",
			sysconfig: syslinuxCfgUbuntu,
		},
		{
			image:     "debian:9",
			kernel:    "/vmlinuz",
			initrd:    "/initrd.img",
			sysconfig: syslinuxCfgDebian,
		},
		{
			image:     "debian:10",
			kernel:    "/vmlinuz",
			initrd:    "/initrd.img",
			sysconfig: syslinuxCfgDebian,
		},
		{
			image:     "debian:11",
			kernel:    "/vmlinuz",
			initrd:    "/initrd.img",
			sysconfig: syslinuxCfgDebian,
		},
		{
			image:     "debian:latest",
			kernel:    "/vmlinuz",
			initrd:    "/initrd.img",
			sysconfig: syslinuxCfgDebian,
		},
		{
			image:     "alpine",
			kernel:    "/boot/vmlinuz-virt",
			initrd:    "/boot/initramfs-virt",
			sysconfig: syslinuxCfgAlpine,
		},
		{
			image:     "centos:8",
			kernel:    "/boot/vmlinuz",
			initrd:    "/boot/initrd.img",
			sysconfig: syslinuxCfgCentOS,
		},
		{
			image:     "centos:latest",
			kernel:    "/boot/vmlinuz",
			initrd:    "/boot/initrd.img",
			sysconfig: syslinuxCfgCentOS,
		},
	}
	exec.SetDebug(true)

	for _, test := range tests {
		test := test
		t.Run(test.image, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			testSysconfig(t, ctx, test.image, test.sysconfig, test.kernel, test.initrd)
		})
	}
}
