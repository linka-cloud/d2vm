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

package d2vm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"go.linka.cloud/d2vm/pkg/docker"
	"go.linka.cloud/d2vm/pkg/qemu_img"
)

const (
	// https://kubevirt.io/user-guide/virtual_machines/disks_and_volumes/#containerdisk-workflow-example
	uid                     = 107
	containerDiskDockerfile = `FROM scratch

ADD --chown=%[1]d:%[1]d %[2]s /disk/
`
)

func MakeContainerDisk(ctx context.Context, path string, tag string) error {
	tmpPath := filepath.Join(os.TempDir(), "d2vm", uuid.New().String())
	if err := os.MkdirAll(tmpPath, os.ModePerm); err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(tmpPath); err != nil {
			logrus.Errorf("failed to remove tmp dir %s: %v", tmpPath, err)
		}
	}()
	if _, err := os.Stat(path); err != nil {
		return err
	}
	// convert may not be needed, but this will also copy the file in the tmp dir
	qcow2 := filepath.Join(tmpPath, "disk.qcow2")
	if err := qemu_img.Convert(ctx, "qcow2", path, qcow2); err != nil {
		return err
	}
	disk := filepath.Base(qcow2)
	dockerfileContent := fmt.Sprintf(containerDiskDockerfile, uid, disk)
	dockerfile := filepath.Join(tmpPath, "Dockerfile")
	if err := os.WriteFile(dockerfile, []byte(dockerfileContent), os.ModePerm); err != nil {
		return fmt.Errorf("failed to write dockerfile: %w", err)
	}
	if err := docker.Build(ctx, tag, dockerfile, tmpPath); err != nil {
		return fmt.Errorf("failed to build container disk: %w", err)
	}
	return nil
}
