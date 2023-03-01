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
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"go.linka.cloud/d2vm"
	"go.linka.cloud/d2vm/pkg/exec"
)

var (
	verbose    = false
	timeFormat = ""
	format     = "qcow2"

	rootCmd = &cobra.Command{
		Use:          "d2vm",
		SilenceUsage: true,
		Version:      d2vm.Version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			switch timeFormat {
			case "full", "f":
			case "relative", "rel", "r":
			case "none", "":
			default:
				logrus.Fatalf("invalid time format: %s. Valid format: 'relative', 'full'", timeFormat)
			}
			if verbose {
				logrus.SetLevel(logrus.TraceLevel)
			}
			exec.SetDebug(verbose)

			// make the zsh completion work when sourced with `source <(d2vm completion zsh)`
			if cmd.Name() == "zsh" && cmd.Parent() != nil && cmd.Parent().Name() == "completion" {
				zshHead := fmt.Sprintf("#compdef %[1]s\ncompdef _%[1]s %[1]s\n", cmd.Root().Name())
				cmd.OutOrStdout().Write([]byte(zshHead))
			}
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
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		logrus.Fatal(err)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "debug", "d", false, "Enable Debug output")
	rootCmd.PersistentFlags().MarkDeprecated("debug", "use -v instead")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable Verbose output")
	rootCmd.PersistentFlags().StringVar(&timeFormat, "time", "none", "Enable formated timed output, valide formats: 'relative (rel | r)', 'full (f)'")
	color.NoColor = false
	logrus.StandardLogger().Formatter = &logfmtFormatter{start: time.Now()}
}

const (
	red    = 31
	yellow = 33
	blue   = 36
	white  = 39
	gray   = 90
)

type logfmtFormatter struct {
	start time.Time
}

func (f *logfmtFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b bytes.Buffer
	var c *color.Color
	switch entry.Level {
	case logrus.DebugLevel, logrus.TraceLevel:
		c = color.New(gray)
	case logrus.WarnLevel:
		c = color.New(yellow)
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		c = color.New(red)
	default:
		c = color.New(white)
	}
	msg := entry.Message
	if len(entry.Message) > 0 && entry.Level < logrus.DebugLevel {
		msg = strings.ToTitle(string(msg[0])) + msg[1:]
	}

	var err error
	switch timeFormat {
	case "full", "f":
		_, err = c.Fprintf(&b, "[%s] %s\n", entry.Time.Format("2006-01-02 15:04:05"), entry.Message)
	case "relative", "rel", "r":
		_, err = c.Fprintf(&b, "[%5v] %s\n", entry.Time.Sub(f.start).Truncate(time.Second).String(), msg)
	default:
		_, err = c.Fprintln(&b, msg)
	}
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func isRoot() bool {
	return os.Geteuid() == 0
}

func sudoUser() (uid int, sudo bool) {
	// if we are not running on linux, docker handle files user's permissions,
	// so we don't need to check for sudo here
	if runtime.GOOS != "linux" {
		return
	}
	v := os.Getenv("SUDO_UID")
	if v == "" {
		return 0, false
	}
	uid, err := strconv.Atoi(v)
	if err != nil {
		logrus.Errorf("invalid SUDO_UID: %s", v)
		return 0, false
	}
	return uid, true
}
