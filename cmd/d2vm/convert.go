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
	raw          bool
	pull         = false
	cmdLineExtra = ""

	convertCmd = &cobra.Command{
		Use:          "convert [docker image]",
		Short:        "Convert Docker image to vm image",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if runtime.GOOS != "linux" {
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
			img := args[0]
			tag := "latest"
			if parts := strings.Split(img, ":"); len(parts) > 1 {
				img, tag = parts[0], parts[1]
			}
			size, err := parseSize(size)
			if err != nil {
				return err
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
				found = len(imgs) == 1 && imgs[0] == fmt.Sprintf("%s:%s", img, tag)
				if found {
					logrus.Infof("using local image %s:%s", img, tag)
				}
			}
			if pull || !found {
				logrus.Infof("pulling image %s", img)
				if err := docker.Pull(cmd.Context(), img); err != nil {
					return err
				}
			}
			return d2vm.Convert(
				cmd.Context(),
				img,
				d2vm.WithSize(size),
				d2vm.WithPassword(password),
				d2vm.WithOutput(output),
				d2vm.WithCmdLineExtra(cmdLineExtra),
				d2vm.WithNetworkManager(d2vm.NetworkManager(networkManager)),
				d2vm.WithRaw(raw),
			)
		},
	}
)

func parseSize(s string) (int64, error) {
	var v datasize.ByteSize
	if err := v.UnmarshalText([]byte(s)); err != nil {
		return 0, err
	}
	return int64(v), nil
}

func init() {
	convertCmd.Flags().BoolVar(&pull, "pull", false, "Always pull docker image")
	convertCmd.Flags().StringVarP(&output, "output", "o", output, "The output image, the extension determine the image format, raw will be used if none. Supported formats: "+strings.Join(d2vm.OutputFormats(), " "))
	convertCmd.Flags().StringVarP(&password, "password", "p", "root", "The Root user password")
	convertCmd.Flags().StringVarP(&size, "size", "s", "10G", "The output image size")
	convertCmd.Flags().BoolVarP(&force, "force", "f", false, "Override output qcow2 image")
	convertCmd.Flags().StringVar(&cmdLineExtra, "append-to-cmdline", "", "Extra kernel cmdline arguments to append to the generated one")
	convertCmd.Flags().StringVar(&networkManager, "network-manager", "", "Network manager to use for the image: none, netplan, ifupdown")
	convertCmd.Flags().BoolVar(&raw, "raw", false, "Just convert the container to virtual machine image without installing anything more")
	rootCmd.AddCommand(convertCmd)
}
