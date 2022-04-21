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
	"io"
	"text/template"
)

const (
	dockerImageRun = `
#!/bin/sh

{{- range .Config.Env }}
export {{ . }}
{{- end }}

cd {{- if .Config.WorkingDir }}{{ .Config.WorkingDir }}{{- else }}/{{- end }}

{{ .Config.Entrypoint }} {{ .Config.Args }}
`
)

var (
	dockerImageRunTemplate = template.Must(template.New("docker-run.sh").Parse(dockerImageRun))
)

type DockerImage struct {
	Config struct {
		Hostname     string `json:"Hostname"`
		Domainname   string `json:"Domainname"`
		User         string `json:"User"`
		AttachStdin  bool   `json:"AttachStdin"`
		AttachStdout bool   `json:"AttachStdout"`
		AttachStderr bool   `json:"AttachStderr"`
		ExposedPorts struct {
			Tcp struct {
			} `json:"3000/tcp"`
		} `json:"ExposedPorts"`
		Tty        bool        `json:"Tty"`
		OpenStdin  bool        `json:"OpenStdin"`
		StdinOnce  bool        `json:"StdinOnce"`
		Env        []string    `json:"Env"`
		Cmd        []string    `json:"Cmd"`
		Image      string      `json:"Image"`
		Volumes    interface{} `json:"Volumes"`
		WorkingDir string      `json:"WorkingDir"`
		Entrypoint []string    `json:"Entrypoint"`
		OnBuild    interface{} `json:"OnBuild"`
		Labels     interface{} `json:"Labels"`
	} `json:"Config"`
	Architecture string `json:"Architecture"`
	Os           string `json:"Os"`
	Size         int    `json:"Size"`
	VirtualSize  int    `json:"VirtualSize"`
}

func (i DockerImage) AsRunScript(w io.Writer) error {
	return dockerImageRunTemplate.Execute(w, i)
}
