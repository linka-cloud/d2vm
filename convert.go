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

package d2vm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/svenwiltink/sparsecat"

	"go.linka.cloud/d2vm/pkg/docker"
)

func Convert(ctx context.Context, img string, opts ...ConvertOption) error {
	o := &convertOptions{}
	for _, opt := range opts {
		opt(o)
	}
	imgUUID := uuid.New().String()
	tmpPath := filepath.Join(os.TempDir(), "d2vm", imgUUID)
	if err := os.MkdirAll(tmpPath, os.ModePerm); err != nil {
		return err
	}
	defer os.RemoveAll(tmpPath)

	logrus.Infof("inspecting image %s", img)
	r, err := FetchDockerImageOSRelease(ctx, img)
	if err != nil {
		return err
	}

	if o.luksPassword != "" && !r.SupportsLUKS() {
		return fmt.Errorf("luks is not supported for %s %s", r.Name, r.Version)
	}

	if !o.raw {
		d, err := NewDockerfile(r, img, o.password, o.networkManager, o.luksPassword != "", o.hasGrubBIOS(), o.hasGrubEFI())
		if err != nil {
			return err
		}
		logrus.Infof("docker image based on %s %s", d.Release.Name, d.Release.Version)
		p := filepath.Join(tmpPath, docker.FormatImgName(img))
		dir := filepath.Dir(p)
		f, err := os.Create(p)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := d.Render(f); err != nil {
			return err
		}
		logrus.Infof("building kernel enabled image")
		if err := docker.Build(ctx, o.pull, imgUUID, p, dir, o.platform); err != nil {
			return err
		}
		if !o.keepCache {
			defer docker.Remove(ctx, imgUUID)
		}
	} else {
		// for raw images, we just tag the image with the uuid
		if err := docker.Tag(ctx, img, imgUUID); err != nil {
			return err
		}
		if !o.keepCache {
			defer docker.Remove(ctx, imgUUID)
		}
	}

	logrus.Infof("creating vm image")
	format := strings.TrimPrefix(filepath.Ext(o.output), ".")
	if format == "" {
		format = "raw"
	} else if format == "vhd" {
		format = "vpc"
	}
	b, err := NewBuilder(ctx, tmpPath, imgUUID, "", o.size, r, format, o.cmdLineExtra, o.splitBoot, o.bootFS, o.bootSize, o.luksPassword, o.bootLoader, o.platform)
	if err != nil {
		return err
	}
	defer b.Close()
	if err := b.Build(ctx); err != nil {
		return err
	}
	if err := os.RemoveAll(o.output); err != nil {
		return err
	}
	if err := MoveFile(filepath.Join(tmpPath, "disk0."+format), o.output); err != nil {
		return err
	}
	return nil
}

func MoveFile(sourcePath, destPath string) error {
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %s", err)
	}
	outputFile, err := os.Create(destPath)
	if err != nil {
		inputFile.Close()
		return fmt.Errorf("failed to open dest file: %s", err)
	}
	defer outputFile.Close()
	_, err = sparsecat.NewDecoder(sparsecat.NewEncoder(inputFile)).WriteTo(outputFile)
	inputFile.Close()
	if err != nil {
		return fmt.Errorf("failed to write to output file: %s", err)
	}
	// The copy was successful, so now delete the original file
	err = os.Remove(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to remove original file: %s", err)
	}
	return nil
}
