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

	"github.com/sirupsen/logrus"
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

type NetworkManager string

const (
	NetworkManagerNone      NetworkManager = "none"
	NetworkManagerIfupdown2 NetworkManager = "ifupdown"
	NetworkManagerNetplan   NetworkManager = "netplan"
)

func (n NetworkManager) Validate() error {
	switch n {
	case NetworkManagerNone, NetworkManagerIfupdown2, NetworkManagerNetplan:
		return nil
	default:
		return fmt.Errorf("unsupported network manager: %s", n)
	}
}

type Dockerfile struct {
	Image          string
	Password       string
	Release        OSRelease
	NetworkManager NetworkManager
	tmpl           *template.Template
}

func (d Dockerfile) Render(w io.Writer) error {
	return d.tmpl.Execute(w, d)
}

func NewDockerfile(release OSRelease, img, password string, networkManager NetworkManager) (Dockerfile, error) {
	if password == "" {
		password = "root"
	}
	d := Dockerfile{Release: release, Image: img, Password: password, NetworkManager: networkManager}
	var net NetworkManager
	switch release.ID {
	case ReleaseDebian:
		d.tmpl = debianDockerfileTemplate
		net = NetworkManagerIfupdown2
	case ReleaseUbuntu:
		d.tmpl = ubuntuDockerfileTemplate
		net = NetworkManagerNetplan
	case ReleaseAlpine:
		d.tmpl = alpineDockerfileTemplate
		net = NetworkManagerIfupdown2
		if networkManager == NetworkManagerNetplan {
			return d, fmt.Errorf("netplan is not supported on alpine")
		}
	case ReleaseCentOS:
		d.tmpl = centOSDockerfileTemplate
		net = NetworkManagerNone
		if networkManager != "" && networkManager != NetworkManagerNone {
			return Dockerfile{}, fmt.Errorf("network manager is not supported on centos")
		}
	default:
		return Dockerfile{}, fmt.Errorf("unsupported distribution: %s", release.ID)
	}
	if d.NetworkManager == "" {
		if release.ID != ReleaseCentOS {
			logrus.Warnf("no network manager specified, using distribution defaults: %s", net)
		}
		d.NetworkManager = net
	}
	if err := d.NetworkManager.Validate(); err != nil {
		return Dockerfile{}, err
	}
	return d, nil
}
