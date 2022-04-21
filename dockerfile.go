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
	_ "embed"
	"fmt"
	"io"
	"text/template"
)

//go:embed templates/ubuntu.Dockerfile
var ubuntuDockerfile string

//go:embed templates/debian.Dockerfile
var debianDockerfile string

//go:embed templates/alpine.Dockerfile
var alpineDockerfile string

//go:embed templates/centos.Dockerfile
var centOSDockerfile string

var (
	ubuntuDockerfileTemplate = template.Must(template.New("ubuntu.Dockerfile").Parse(ubuntuDockerfile))
	debianDockerfileTemplate = template.Must(template.New("debian.Dockerfile").Parse(debianDockerfile))
	alpineDockerfileTemplate = template.Must(template.New("alpine.Dockerfile").Parse(alpineDockerfile))
	centOSDockerfileTemplate = template.Must(template.New("centos.Dockerfile").Parse(centOSDockerfile))
)

type Dockerfile struct {
	Image    string
	Password string
	Release  OSRelease
	tmpl     *template.Template
}

func (d Dockerfile) Render(w io.Writer) error {
	return d.tmpl.Execute(w, d)
}

func NewDockerfile(release OSRelease, img, password string) (Dockerfile, error) {
	if password == "" {
		password = "root"
	}
	d := Dockerfile{Release: release, Image: img, Password: password}
	switch release.ID {
	case ReleaseDebian:
		d.tmpl = debianDockerfileTemplate
	case ReleaseUbuntu:
		d.tmpl = ubuntuDockerfileTemplate
	case ReleaseAlpine:
		d.tmpl = alpineDockerfileTemplate
	case ReleaseCentOS, ReleaseRHEL:
		d.tmpl = centOSDockerfileTemplate
	default:
		return Dockerfile{}, fmt.Errorf("unsupported distribution: %s", release.ID)
	}
	return d, nil
}
