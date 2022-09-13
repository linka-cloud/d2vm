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

package exec

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

var (
	Run = RunNoOut
)

func SetDebug(debug bool) {
	if debug {
		Run = RunDebug
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		Run = RunNoOut
	}
}

func CommandContext(ctx context.Context, c string, args ...string) *exec.Cmd {
	logrus.Debugf("$ %s %s", c, strings.Join(args, " "))
	return exec.CommandContext(ctx, c, args...)
}

func RunDebug(ctx context.Context, c string, args ...string) error {
	logrus.Debugf("$ %s %s", c, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, c, args...)
	cmd.Stdout = logrus.StandardLogger().WriterLevel(logrus.DebugLevel)
	cmd.Stderr = logrus.StandardLogger().WriterLevel(logrus.DebugLevel)
	return cmd.Run()
}

func RunNoOut(ctx context.Context, c string, args ...string) error {
	_, _, err := RunOut(ctx, c, args...)
	if err != nil {
		return err
	}
	return nil
}

func RunOut(ctx context.Context, c string, args ...string) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, c, args...)
	var stdo, stde bytes.Buffer
	cmd.Stdout = &stdo
	cmd.Stderr = &stde
	err = cmd.Run()
	if err != nil {
		return "", "", fmt.Errorf("%s %s: stdout: %s stderr: %s error: %w", c, strings.Join(args, " "), stdo.String(), stde.String(), err)
	}
	return stdo.String(), stde.String(), nil
}
