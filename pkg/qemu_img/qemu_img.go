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

package qemu_img

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"

	"go.linka.cloud/d2vm/pkg/docker"
	exec2 "go.linka.cloud/d2vm/pkg/exec"
)

var (
	DockerImageName    string
	DockerImageVersion string
)

type ImgInfo struct {
	VirtualSize int    `json:"virtual-size"`
	Filename    string `json:"filename"`
	Format      string `json:"format"`
	ActualSize  int    `json:"actual-size"`
	DirtyFlag   bool   `json:"dirty-flag"`
}

func Info(ctx context.Context, in string) (*ImgInfo, error) {
	var (
		o   []byte
		err error
	)
	if path, _ := exec.LookPath("qemu-img"); path == "" {
		inAbs, err := filepath.Abs(in)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for %q: %v", path, err)
		}
		inMount := filepath.Dir(inAbs)
		in := filepath.Join("/in", filepath.Base(inAbs))
		o, err = exec2.CommandContext(
			ctx,
			"docker",
			"run",
			"--rm",
			"-v",
			inMount+":/in",
			"--entrypoint",
			"qemu-img",
			fmt.Sprintf("%s:%s", DockerImageName, DockerImageVersion),
			"info",
			in,
			"--output",
			"json",
		).CombinedOutput()
	} else {
		o, err = exec2.CommandContext(ctx, "qemu-img", "info", in, "--output", "json").CombinedOutput()
	}
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, string(o))
	}
	var i ImgInfo
	if err := json.Unmarshal(o, &i); err != nil {
		return nil, err
	}
	return &i, nil
}

func Convert(ctx context.Context, format, in, out string) error {
	if path, _ := exec.LookPath("qemu-img"); path != "" {
		return exec2.Run(ctx, "qemu-img", "convert", "-O", format, in, out)
	}
	inAbs, err := filepath.Abs(in)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %q: %v", in, err)
	}
	inMount := filepath.Dir(inAbs)
	in = filepath.Join("/in", filepath.Base(inAbs))

	outAbs, err := filepath.Abs(out)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %q: %v", out, err)
	}
	outMount := filepath.Dir(outAbs)
	out = filepath.Join("/out", filepath.Base(outAbs))

	return docker.RunAndRemove(
		ctx,
		"-v",
		fmt.Sprintf("%s:/in", inMount),
		"-v",
		fmt.Sprintf("%s:/out", outMount),
		"--entrypoint",
		"qemu-img",
		fmt.Sprintf("%s:%s", DockerImageName, DockerImageVersion),
		"convert",
		"-O",
		format,
		in,
		out,
	)
}
