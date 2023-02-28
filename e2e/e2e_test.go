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

package e2e

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	require2 "github.com/stretchr/testify/require"

	"go.linka.cloud/d2vm"
	"go.linka.cloud/d2vm/pkg/docker"
	"go.linka.cloud/d2vm/pkg/qemu"
)

type test struct {
	name string
	args []string
}

type img struct {
	name string
	luks string
}

var images = []img{
	{name: "alpine:3.17", luks: "Enter passphrase for /dev/sda2:"},
	{name: "ubuntu:20.04", luks: "Please unlock disk root:"},
	{name: "debian:11", luks: "Please unlock disk root:"},
	{name: "centos:8", luks: "Please enter passphrase for disk"},
}

func TestConvert(t *testing.T) {
	require := require2.New(t)
	tests := []test{
		{
			name: "single-partition",
		},
		{
			name: "split-boot",
			args: []string{"--split-boot"},
		},
		{
			name: "luks",
			args: []string{"--luks-password=root"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Parallel()
			dir := filepath.Join("/tmp", "d2vm-e2e", tt.name)
			require.NoError(os.MkdirAll(dir, os.ModePerm))

			defer os.RemoveAll(dir)
			for _, img := range images {
				t.Run(img.name, func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					// t.Parallel()
					require := require2.New(t)

					out := filepath.Join(dir, strings.NewReplacer(":", "-", ".", "-").Replace(img.name)+".qcow2")

					if _, err := os.Stat(out); err == nil {
						require.NoError(os.Remove(out))
					}
					
					require.NoError(docker.RunD2VM(ctx, d2vm.Image, d2vm.Version, dir, dir, "convert", append([]string{"-p", "root", "-o", "/out/" + filepath.Base(out), "-v", "--keep-cache", img.name}, tt.args...)...))

					inr, inw := io.Pipe()
					defer inr.Close()
					outr, outw := io.Pipe()
					defer outw.Close()
					var success atomic.Bool
					go func() {
						time.AfterFunc(2*time.Minute, cancel)
						defer inw.Close()
						defer outr.Close()
						login := []byte("login:")
						password := []byte("Password:")
						s := bufio.NewScanner(outr)
						s.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
							if i := bytes.Index(data, []byte(img.luks)); i >= 0 {
								return i + len(img.luks), []byte(img.luks), nil
							}
							if i := bytes.Index(data, login); i >= 0 {
								return i + len(login), login, nil
							}
							if i := bytes.Index(data, password); i >= 0 {
								return i + len(password), password, nil
							}
							if atEOF {
								return 0, nil, io.EOF
							}
							return 0, nil, nil
						})
						for s.Scan() {
							b := s.Bytes()
							if bytes.Contains(b, []byte(img.luks)) {
								t.Logf("sending luks password")
								if _, err := inw.Write([]byte("root\n")); err != nil {
									t.Logf("failed to write luks password: %v", err)
									cancel()
								}
							}
							if bytes.Contains(b, login) {
								t.Logf("sending login")
								if _, err := inw.Write([]byte("root\n")); err != nil {
									t.Logf("failed to write login: %v", err)
									cancel()
								}
							}
							if bytes.Contains(b, password) {
								t.Logf("sending password")
								if _, err := inw.Write([]byte("root\n")); err != nil {
									t.Logf("failed to write password: %v", err)
									cancel()
								}
								time.Sleep(time.Second)
								if _, err := inw.Write([]byte("poweroff\n")); err != nil {
									t.Logf("failed to write poweroff: %v", err)
									cancel()
								}
								success.Store(true)
								return
							}
						}
						if err := s.Err(); err != nil {
							t.Logf("failed to scan output: %v", err)
							cancel()
						}
					}()
					if err := qemu.Run(ctx, out, qemu.WithStdin(inr), qemu.WithStdout(io.MultiWriter(outw, os.Stdout)), qemu.WithStderr(io.Discard), qemu.WithMemory(2048)); err != nil && !success.Load() {
						t.Fatalf("failed to run qemu: %v", err)
					}
				})
			}
		})
	}
}
