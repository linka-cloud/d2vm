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
	"strconv"
	"strings"
	"text/template"

	"github.com/sirupsen/logrus"
)

//go:embed templates/ubuntu.Dockerfile
var ubuntuDockerfile string

//go:embed templates/ubuntu12.Dockerfile
var ubuntu12Dockerfile string

//go:embed templates/debian.Dockerfile
var debianDockerfile string

//go:embed templates/alpine.Dockerfile
var alpineDockerfile string

//go:embed templates/centos.Dockerfile
var centOSDockerfile string

var (
	ubuntuDockerfileTemplate   = template.Must(template.New("ubuntu.Dockerfile").Funcs(tplFuncs).Parse(ubuntuDockerfile))
	ubuntu12DockerfileTemplate = template.Must(template.New("ubuntu12.Dockerfile").Funcs(tplFuncs).Parse(ubuntu12Dockerfile))
	debianDockerfileTemplate   = template.Must(template.New("debian.Dockerfile").Funcs(tplFuncs).Parse(debianDockerfile))
	alpineDockerfileTemplate   = template.Must(template.New("alpine.Dockerfile").Funcs(tplFuncs).Parse(alpineDockerfile))
	centOSDockerfileTemplate   = template.Must(template.New("centos.Dockerfile").Funcs(tplFuncs).Parse(centOSDockerfile))
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
	Luks           bool
	GrubBIOS       bool
	GrubEFI        bool
	tmpl           *template.Template
}

func (d Dockerfile) Grub() bool {
	return d.GrubBIOS || d.GrubEFI
}

func (d Dockerfile) Render(w io.Writer) error {
	return d.tmpl.Execute(w, d)
}

func NewDockerfile(release OSRelease, img, password string, networkManager NetworkManager, luks, grubBIOS, grubEFI bool) (Dockerfile, error) {
	d := Dockerfile{Release: release, Image: img, Password: password, NetworkManager: networkManager, Luks: luks, GrubBIOS: grubBIOS, GrubEFI: grubEFI}
	var net NetworkManager
	switch release.ID {
	case ReleaseDebian:
		d.tmpl = debianDockerfileTemplate
		net = NetworkManagerIfupdown2
	case ReleaseKali:
		d.tmpl = debianDockerfileTemplate
		net = NetworkManagerIfupdown2
	case ReleaseUbuntu:
		if strings.HasPrefix(release.VersionID, "12.") {
			d.tmpl = ubuntu12DockerfileTemplate
		} else {
			d.tmpl = ubuntuDockerfileTemplate
		}
		if release.VersionID < "18.04" {
			net = NetworkManagerIfupdown2
		} else {
			net = NetworkManagerNetplan
		}
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

var tplFuncs = template.FuncMap{
	"atoi": strconv.Atoi,
}
