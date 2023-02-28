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
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/google/go-containerregistry/cmd/crane/cmd"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"

	"go.linka.cloud/d2vm/pkg/docker"
	"go.linka.cloud/d2vm/pkg/exec"
)

const (
	dockerImageRun = `
#!/bin/sh

{{- range .DockerImageConfig.Env }}
export {{ . }}
{{- end }}

{{ if .DockerImageConfig.WorkingDir }}cd {{ .DockerImageConfig.WorkingDir }}{{ end }}

{{ if .DockerImageConfig.User }}su {{ .DockerImageConfig.User }} -p -s /bin/sh -c '{{ end }}{{ if .DockerImageConfig.Entrypoint}}{{ format .DockerImageConfig.Entrypoint }} {{ end}}{{ if .DockerImageConfig.Cmd }}{{ format .DockerImageConfig.Cmd }}{{ end }}{{ if .DockerImageConfig.User }}'{{- end }}
`
)

var (
	dockerImageRunTemplate = template.Must(template.New("docker-run.sh").Funcs(map[string]interface{}{"format": func(a []string) string {
		var o []string
		for _, v := range a {
			o = append(o, fmt.Sprintf("\"%s\"", v))
		}
		return strings.Join(o, " ")
	}}).Parse(dockerImageRun))

	_ = cmd.NewCmdFlatten
)

type DockerImage struct {
	DockerImageConfig `json:"Config"`
	Architecture      string `json:"Architecture"`
	Os                string `json:"Os"`
	Size              int    `json:"Size"`
}

type DockerImageConfig struct {
	Image      string   `json:"Image"`
	Hostname   string   `json:"Hostname"`
	Domainname string   `json:"Domainname"`
	User       string   `json:"User"`
	Env        []string `json:"Env"`
	Cmd        []string `json:"Cmd"`
	WorkingDir string   `json:"WorkingDir"`
	Entrypoint []string `json:"Entrypoint"`
}

func (i DockerImage) AsRunScript(w io.Writer) error {
	return dockerImageRunTemplate.Execute(w, i)
}

func NewImage(ctx context.Context, tag string, imageTmpPath string) (*image, error) {
	if err := os.MkdirAll(imageTmpPath, perm); err != nil {
		return nil, err
	}
	// save the image to a tar file to avoid loading it in memory
	tar := filepath.Join(imageTmpPath, "img.layers.tar")
	if err := docker.ImageSave(ctx, tag, tar); err != nil {
		return nil, err
	}
	img, err := crane.Load(tar)
	if err != nil {
		return nil, err
	}
	i := &image{
		img: img,
		dir: imageTmpPath,
	}
	return i, nil
}

type image struct {
	tag      string
	img      v1.Image
	dir      string
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}

func (i image) Flatten(ctx context.Context, out string) error {
	tar := filepath.Join(i.dir, "img.tar")
	f, err := os.Create(tar)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, mutate.Extract(i.img)); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := exec.Run(ctx, "tar", "xvf", tar, "-C", out); err != nil {
		return err
	}
	return nil
}

func (i image) Close() error {
	return os.RemoveAll(i.dir)
}
