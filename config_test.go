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
	"github.com/stretchr/testify/require"

	"go.linka.cloud/d2vm/pkg/docker"
	"go.linka.cloud/d2vm/pkg/exec"
)

func testConfig(t *testing.T, ctx context.Context, img string, config Config) {
	require.NoError(t, docker.Pull(ctx, img))
	tmpPath := filepath.Join(os.TempDir(), "d2vm-tests", strings.NewReplacer(":", "-", ".", "-").Replace(img))
	require.NoError(t, os.MkdirAll(tmpPath, 0755))
	defer os.RemoveAll(tmpPath)
	logrus.Infof("inspecting image %s", img)
	r, err := FetchDockerImageOSRelease(ctx, img, tmpPath)
	require.NoError(t, err)
	defer docker.Remove(ctx, img)
	d, err := NewDockerfile(r, img, "root", "", false, false)
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
	require.NoError(t, docker.RunAndRemove(ctx, imgUUID, "test", "-f", config.Kernel))
	require.NoError(t, docker.RunAndRemove(ctx, imgUUID, "test", "-f", config.Initrd))
}

func TestConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		image  string
		config Config
	}{
		{
			image:  "ubuntu:18.04",
			config: configDebian,
		},
		{
			image:  "ubuntu:20.04",
			config: configUbuntu,
		},
		{
			image:  "ubuntu:22.04",
			config: configUbuntu,
		},
		{
			image:  "ubuntu:latest",
			config: configUbuntu,
		},
		{
			image:  "debian:9",
			config: configDebian,
		},
		{
			image:  "debian:10",
			config: configDebian,
		},
		{
			image:  "debian:11",
			config: configDebian,
		},
		{
			image:  "debian:latest",
			config: configDebian,
		},
		{
			image:  "kalilinux/kali-rolling:latest",
			config: configDebian,
		},
		{
			image:  "alpine:3.16",
			config: configAlpine,
		},
		{
			image:  "alpine",
			config: configAlpine,
		},
		{
			image:  "centos:8",
			config: configCentOS,
		},
		{
			image:  "centos:latest",
			config: configCentOS,
		},
	}
	exec.SetDebug(true)

	for _, test := range tests {
		test := test
		t.Run(test.image, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			testConfig(t, ctx, test.image, test.config)
		})
	}
}
