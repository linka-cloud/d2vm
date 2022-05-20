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

package main

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"go.linka.cloud/d2vm/cmd/d2vm/run"
)

var (
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "run the converted virtual machine",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.AddCommand(run.VboxCmd)
	runCmd.AddCommand(run.QemuCmd)
	runCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable Debug output")
}
