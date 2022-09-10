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
	"text/template"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	"go.linka.cloud/d2vm/pkg/docker"
)

const (
	ReleaseUbuntu Release = "ubuntu"
	ReleaseDebian Release = "debian"
	ReleaseAlpine Release = "alpine"
	ReleaseCentOS Release = "centos"
	ReleaseRHEL   Release = "rhel"
)

type Release string

func (r Release) Supported() bool {
	switch r {
	case ReleaseUbuntu:
		return true
	case ReleaseDebian:
		return true
	case ReleaseAlpine:
		return true
	case ReleaseCentOS:
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

const (
	osReleaseDockerfile = `
FROM {{ . }}

ENTRYPOINT [""]

CMD ["/bin/cat", "/etc/os-release"]
`
)

var (
	osReleaseDockerfileTemplate = template.Must(template.New("osrelease.Dockerfile").Parse(osReleaseDockerfile))
)

func FetchDockerImageOSRelease(ctx context.Context, img string, tmpPath string) (OSRelease, error) {
	d := filepath.Join(tmpPath, "osrelease.Dockerfile")
	f, err := os.Create(d)
	if err != nil {
		return OSRelease{}, err
	}
	defer f.Close()
	if err := osReleaseDockerfileTemplate.Execute(f, img); err != nil {
		return OSRelease{}, err
	}
	imgTag := fmt.Sprintf("os-release-%s", img)
	if err := docker.Cmd(ctx, "image", "build", "-t", imgTag, "-f", d, tmpPath); err != nil {
		return OSRelease{}, err
	}
	defer func() {
		if err := docker.Cmd(ctx, "image", "rm", imgTag); err != nil {
			logrus.WithError(err).Error("failed to cleanup OSRelease Docker Image")
		}
	}()
	o, _, err := docker.CmdOut(ctx, "run", "--rm", "-i", imgTag)
	if err != nil {
		return OSRelease{}, err
	}
	return ParseOSRelease(o)
}
