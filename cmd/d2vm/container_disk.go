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
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"go.linka.cloud/d2vm"
	"go.linka.cloud/d2vm/pkg/docker"
)

func maybeMakeContainerDisk(ctx context.Context) error {
	if containerDiskTag == "" {
		return nil
	}
	logrus.Infof("creating container disk image %s", containerDiskTag)
	if err := d2vm.MakeContainerDisk(ctx, output, containerDiskTag, platform); err != nil {
		return err
	}
	if !push {
		return nil
	}
	logrus.Infof("pushing container disk image %s", containerDiskTag)
	if err := docker.Push(ctx, containerDiskTag); err != nil {
		return fmt.Errorf("failed to push container disk: %w", err)
	}
	return nil
}
