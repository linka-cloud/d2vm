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
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"go.linka.cloud/d2vm"
	"go.linka.cloud/d2vm/pkg/exec"
)

var (
	output   = "disk0.qcow2"
	size     = "1G"
	password = "root"
	force    = false
	verbose  = false
	format   = "qcow2"

	rootCmd = &cobra.Command{
		Use:          "d2vm",
		SilenceUsage: true,
		Version:      d2vm.Version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if verbose {
				logrus.SetLevel(logrus.TraceLevel)
			}
			exec.SetDebug(verbose)
		},
	}
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, os.Kill)
	go func() {
		<-sigs
		fmt.Println()
		cancel()
	}()
	rootCmd.ExecuteContext(ctx)
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "debug", "d", false, "Enable Debug output")
	rootCmd.PersistentFlags().MarkDeprecated("debug", "use -v instead")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable Verbose output")
}
