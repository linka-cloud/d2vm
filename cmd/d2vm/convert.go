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

	"github.com/c2h5oh/datasize"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"go.linka.cloud/d2vm"
	"go.linka.cloud/d2vm/pkg/docker"
)

var (
	convertCmd = &cobra.Command{
		Use:          "convert [docker image]",
		Short:        "Convert Docker image to vm image",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if runtime.GOOS != "linux" || !isRoot() {
				abs, err := filepath.Abs(output)
				if err != nil {
					return err
				}
				out := filepath.Dir(abs)
				dargs := os.Args[2:]
				for i, v := range dargs {
					if v == output {
						dargs[i] = filepath.Join("/out", filepath.Base(output))
						break
					}
				}
				return docker.RunD2VM(cmd.Context(), d2vm.Image, d2vm.Version, out, out, cmd.Name(), dargs...)
			}
			if luksPassword != "" && !splitBoot {
				logrus.Warnf("luks password is set: enabling split boot")
				splitBoot = true
			}
			img := args[0]
			tag := "latest"
			if parts := strings.Split(img, ":"); len(parts) > 1 {
				img, tag = parts[0], parts[1]
			}
			img = fmt.Sprintf("%s:%s", img, tag)
			size, err := parseSize(size)
			if err != nil {
				return err
			}
			if push && tag == "" {
				return fmt.Errorf("tag is required when pushing container disk image")
			}
			if _, err := os.Stat(output); err == nil || !os.IsNotExist(err) {
				if !force {
					return fmt.Errorf("%s already exists", output)
				}
			}
			found := false
			if !pull {
				imgs, err := docker.ImageList(cmd.Context(), img)
				if err != nil {
					return err
				}
				found = len(imgs) == 1 && imgs[0] == img
				if found {
					logrus.Infof("using local image %s", img)
				}
			}
			if pull || !found {
				logrus.Infof("pulling image %s", img)
				if err := docker.Pull(cmd.Context(), img); err != nil {
					return err
				}
			}
			if err := d2vm.Convert(
				cmd.Context(),
				img,
				d2vm.WithSize(size),
				d2vm.WithPassword(password),
				d2vm.WithOutput(output),
				d2vm.WithCmdLineExtra(cmdLineExtra),
				d2vm.WithNetworkManager(d2vm.NetworkManager(networkManager)),
				d2vm.WithRaw(raw),
				d2vm.WithSplitBoot(splitBoot),
				d2vm.WithBootSize(bootSize),
				d2vm.WithLuksPassword(luksPassword),
				d2vm.WithKeepCache(keepCache),
			); err != nil {
				return err
			}
			// set user permissions on the output file if the command was run with sudo
			if uid, ok := sudoUser(); ok {
				if err := os.Chown(output, uid, uid); err != nil {
					return err
				}
			}
			return maybeMakeContainerDisk(cmd.Context())
		},
	}
)

func parseSize(s string) (uint64, error) {
	var v datasize.ByteSize
	if err := v.UnmarshalText([]byte(s)); err != nil {
		return 0, err
	}
	return uint64(v), nil
}

func init() {
	convertCmd.Flags().BoolVar(&pull, "pull", false, "Always pull docker image")
	convertCmd.Flags().AddFlagSet(buildFlags())
	rootCmd.AddCommand(convertCmd)
}
