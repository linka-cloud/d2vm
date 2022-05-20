// Copyright 2022 Linka Cloud  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package docker

import (
	"bufio"
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"

	"go.linka.cloud/d2vm/pkg/exec"
)

func dockerSocket() string {
	if runtime.GOOS == "windows" {
		return "//var/run/docker.sock"
	}
	return "/var/run/docker.sock"
}

func FormatImgName(name string) string {
	s := strings.Replace(name, ":", "-", -1)
	s = strings.Replace(s, "/", "_", -1)
	return s
}

func Cmd(ctx context.Context, args ...string) error {
	return exec.Run(ctx, "docker", args...)
}

func CmdOut(ctx context.Context, args ...string) (string, string, error) {
	return exec.RunOut(ctx, "docker", args...)
}

func Build(ctx context.Context, tag, dockerfile, dir string, buildArgs ...string) error {
	if dockerfile == "" {
		dockerfile = filepath.Join(dir, "Dockerfile")
	}
	args := []string{"image", "build", "-t", tag, "-f", dockerfile}
	for _, v := range buildArgs {
		args = append(args, "--build-arg", v)
	}
	args = append(args, dir)
	return Cmd(ctx, args...)
}

func Remove(ctx context.Context, tag string) error {
	return Cmd(ctx, "image", "rm", tag)
}

func ImageList(ctx context.Context, tag string) ([]string, error) {
	o, _, err := CmdOut(ctx, "image", "ls", "--format={{ .Repository }}:{{ .Tag }}", tag)
	if err != nil {
		return nil, err
	}
	s := bufio.NewScanner(strings.NewReader(o))
	var imgs []string
	for s.Scan() {
		imgs = append(imgs, s.Text())
	}
	return imgs, s.Err()
}

func Pull(ctx context.Context, tag string) error {
	return Cmd(ctx, "image", "pull", tag)
}

func RunInteractiveAndRemove(ctx context.Context, args ...string) error {
	logrus.Tracef("running 'docker run --rm -i -t %s'", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, "docker", append([]string{"run", "--rm", "-it"}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunD2VM(ctx context.Context, image, version, cmd string, args ...string) error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if version == "" {
		version = "latest"
	}
	a := []string{
		"--privileged",
		"-v",
		fmt.Sprintf("%s:/var/run/docker.sock", dockerSocket()),
		"-v",
		fmt.Sprintf("%s:/d2vm", pwd),
		"-w",
		"/d2vm",
		fmt.Sprintf("%s:%s", image, version),
		cmd,
	}
	return RunInteractiveAndRemove(ctx, append(a, args...)...)
}
