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
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	"go.linka.cloud/d2vm/pkg/docker"
)

const (
	ReleaseUbuntu    Release = "ubuntu"
	ReleaseDebian    Release = "debian"
	ReleaseAlpine    Release = "alpine"
	ReleaseCentOS    Release = "centos"
	ReleaseRHEL      Release = "rhel"
	ReleaseKali      Release = "kali"
	ReleaseRocky     Release = "rocky"
	ReleaseAlmaLinux Release = "almalinux"
)

type Release string

func (r Release) Supported() bool {
	switch r {
	case ReleaseUbuntu:
		return true
	case ReleaseDebian:
		return true
	case ReleaseKali:
		return true
	case ReleaseAlpine:
		return true
	case ReleaseCentOS:
		return true
	case ReleaseRocky:
		return true
	case ReleaseAlmaLinux:
		return true
	case ReleaseRHEL:
		return false
	default:
		return false
	}
}

type OSRelease struct {
	ID              Release
	Name            string
	VersionID       string
	Version         string
	VersionCodeName string
}

func (r OSRelease) SupportsLUKS() bool {
	switch r.ID {
	case ReleaseUbuntu:
		return r.VersionID >= "20.04"
	case ReleaseDebian:
		v, err := strconv.Atoi(r.VersionID)
		if err != nil {
			logrus.Warnf("%s: failed to parse version id: %v", r.Version, err)
			return false
		}
		return v >= 10
	case ReleaseKali:
		// TODO: check version
		return true
	case ReleaseCentOS:
		return true
	case ReleaseRocky:
		return true
	case ReleaseAlmaLinux:
		return true
	case ReleaseAlpine:
		return true
	case ReleaseRHEL:
		return false
	default:
		return false
	}
}

func ParseOSRelease(s string) (OSRelease, error) {
	env, err := godotenv.Parse(strings.NewReader(s))
	if err != nil {
		return OSRelease{}, err
	}
	o := OSRelease{
		ID:              Release(strings.ToLower(env["ID"])),
		Name:            env["NAME"],
		Version:         env["VERSION"],
		VersionID:       env["VERSION_ID"],
		VersionCodeName: env["VERSION_CODENAME"],
	}
	return o, nil
}

func FetchDockerImageOSRelease(ctx context.Context, img string) (OSRelease, error) {
	o, _, err := docker.CmdOut(ctx, "run", "--rm", "-i", "--entrypoint", "cat", img, "/etc/os-release")
	if err != nil {
		return OSRelease{}, err
	}
	return ParseOSRelease(o)
}
