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
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"go.linka.cloud/d2vm"
	"go.linka.cloud/d2vm/pkg/docker"
	"go.linka.cloud/d2vm/pkg/exec"
)

var (
	file      = "Dockerfile"
	tag       = uuid.New().String()
	buildArgs []string
	buildCmd  = &cobra.Command{
		Use:   "build [context directory]",
		Short: "Build a vm image from Dockerfile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			size, err := parseSize(size)
			if err != nil {
				return err
			}
			exec.SetDebug(debug)
			logrus.Infof("building docker image from %s", file)
			if err := docker.Build(cmd.Context(), tag, file, args[0], buildArgs...); err != nil {
				return err
			}
			return d2vm.Convert(cmd.Context(), tag, size, password, output, format)
		},
	}
)

func init() {
	rootCmd.AddCommand(buildCmd)

	buildCmd.Flags().StringVarP(&file, "file", "f", "Dockerfile", "Name of the Dockerfile (Default is 'PATH/Dockerfile')")
	buildCmd.Flags().StringArrayVar(&buildArgs, "build-arg", nil, "Set build-time variables")

	buildCmd.Flags().StringVarP(&format, "output-format", "O", format, "The output image format, supported formats: "+strings.Join(d2vm.OutputFormats(), " "))
	buildCmd.Flags().StringVarP(&output, "output", "o", output, "The output image")
	buildCmd.Flags().StringVarP(&password, "password", "p", "root", "Root user password")
	buildCmd.Flags().StringVarP(&size, "size", "s", "10G", "The output image size")
	buildCmd.Flags().BoolVarP(&debug, "debug", "d", false, "Enable Debug output")
	buildCmd.Flags().BoolVar(&force, "force", false, "Override output image")
}
