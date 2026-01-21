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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"go.linka.cloud/d2vm"
	"go.linka.cloud/d2vm/pkg/docker"
)

var (
	file      = "Dockerfile"
	tag       = "d2vm-" + uuid.New().String()
	buildArgs []string
	buildCmd  = &cobra.Command{
		Use:   "build [context directory]",
		Short: "Build a vm image from Dockerfile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO(adphi): resolve context path
			if runtime.GOOS != "linux" || !isRoot() {
				ctxAbsPath, err := filepath.Abs(args[0])
				if err != nil {
					return err
				}
				if file == "" {
					file = filepath.Join(args[0], "Dockerfile")
				}
				dockerFileAbsPath, err := filepath.Abs(file)
				if err != nil {
					return err
				}
				if !strings.HasPrefix(dockerFileAbsPath, ctxAbsPath) {
					return fmt.Errorf("Dockerfile must be in the context directory path")
				}
				outputPath, err := filepath.Abs(output)
				if err != nil {
					return err
				}
				var (
					in  = ctxAbsPath
					out = filepath.Dir(outputPath)
				)
				dargs := os.Args[2:]
				for i, v := range dargs {
					switch v {
					case file:
						rel, err := filepath.Rel(in, dockerFileAbsPath)
						if err != nil {
							return fmt.Errorf("failed to construct Dockerfile container paths: %w", err)
						}
						dargs[i] = filepath.Join("/in", rel)
					case output:
						dargs[i] = filepath.Join("/out", filepath.Base(output))
					case args[0]:
						dargs[i] = "/in"
					}
				}
				return docker.RunD2VM(cmd.Context(), d2vm.Image, d2vm.Version, in, out, cmd.Name(), os.Args[2:]...)
			}
			if err := validateFlags(); err != nil {
				return err
			}
			size, err := parseSize(size)
			if err != nil {
				return err
			}
			if file == "" {
				file = filepath.Join(args[0], "Dockerfile")
			}
			logrus.Infof("building docker image from %s", file)
			if err := docker.Build(cmd.Context(), pull, tag, file, args[0], platform, buildArgs...); err != nil {
				return err
			}
			if err := d2vm.Convert(
				cmd.Context(),
				tag,
				d2vm.WithSize(size),
				d2vm.WithPassword(password),
				d2vm.WithOutput(output),
				d2vm.WithCmdLineExtra(cmdLineExtra),
				d2vm.WithNetworkManager(d2vm.NetworkManager(networkManager)),
				d2vm.WithBootLoader(bootloader),
				d2vm.WithRaw(raw),
				d2vm.WithSplitBoot(splitBoot),
				d2vm.WithBootSize(bootSize),
				d2vm.WithBootFS(d2vm.BootFS(bootFS)),
				d2vm.WithLuksPassword(luksPassword),
				d2vm.WithKeepCache(keepCache),
				d2vm.WithPlatform(platform),
				d2vm.WithPull(false),
				d2vm.WithHostname(hostname),
				d2vm.WithDNS(dns),
				d2vm.WithDNSSearch(dnsSearch),
			); err != nil {
				return err
			}
			if uid, ok := sudoUser(); ok {
				if err := os.Chown(output, uid, uid); err != nil {
					return err
				}
			}
			return maybeMakeContainerDisk(cmd.Context())
		},
	}
)

func init() {
	rootCmd.AddCommand(buildCmd)

	buildCmd.Flags().StringVarP(&file, "file", "f", "", "Name of the Dockerfile")
	buildCmd.Flags().StringArrayVar(&buildArgs, "build-arg", nil, "Set build-time variables")

	buildCmd.Flags().AddFlagSet(buildFlags())
}
