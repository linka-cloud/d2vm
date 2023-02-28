package d2vm

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.linka.cloud/d2vm/pkg/docker"
	"go.linka.cloud/d2vm/pkg/exec"
)

func TestDockerImageAsRunSript(t *testing.T) {
	tests := []struct {
		name  string
		image DockerImage
		want  string
	}{
		{
			name: "nothing",
			image: DockerImage{
				DockerImageConfig: DockerImageConfig{
					User:       "",
					WorkingDir: "",
					Env:        nil,
					Entrypoint: nil,
					Cmd:        nil,
				},
			},
			want: `
#!/bin/sh




`,
		},
		{
			name: "tail -f /dev/null",
			image: DockerImage{
				DockerImageConfig: DockerImageConfig{
					User: "root",
					Cmd:  []string{"tail", "-f", "/dev/null"},
				},
			},
			want: `
#!/bin/sh



su root -p -s /bin/sh -c '"tail" "-f" "/dev/null"'
`,
		},
		{
			name: "tail -f /dev/null inside home",
			image: DockerImage{
				DockerImageConfig: DockerImageConfig{
					User:       "root",
					WorkingDir: "/root",
					Cmd:        []string{"tail", "-f", "/dev/null"},
				},
			},
			want: `
#!/bin/sh

cd /root

su root -p -s /bin/sh -c '"tail" "-f" "/dev/null"'
`,
		},
		{
			name: "subshell tail -f /dev/null",
			image: DockerImage{
				DockerImageConfig: DockerImageConfig{
					User:       "root",
					Entrypoint: []string{"/bin/sh", "-c"},
					Cmd:        []string{"tail -f /dev/null"},
				},
			},
			want: `
#!/bin/sh



su root -p -s /bin/sh -c '"/bin/sh" "-c" "tail -f /dev/null"'
`,
		},
		{
			name: "www-data with env",
			image: DockerImage{
				DockerImageConfig: DockerImageConfig{
					User: "www-data",
					Cmd:  []string{"tail", "-f", "/dev/null"},
					Env:  []string{"ENV=PROD", "DB=mysql://user:password@localhost"},
				},
			},
			want: `
#!/bin/sh
export ENV=PROD
export DB=mysql://user:password@localhost



su www-data -p -s /bin/sh -c '"tail" "-f" "/dev/null"'
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w bytes.Buffer
			require.NoError(t, tt.image.AsRunScript(&w))
			assert.Equal(t, tt.want, w.String())
		})
	}
}

func TestImageFlatten(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const (
		img        = "d2vm-flatten-test"
		dockerfile = `FROM alpine

COPY resolv.conf /etc/
COPY hostname /etc/

RUN rm -rf /etc/apk
`
	)
	exec.SetDebug(true)
	tmp := filepath.Join(os.TempDir(), "d2vm-tests", "image-flatten")
	require.NoError(t, os.MkdirAll(tmp, os.ModePerm))
	defer os.RemoveAll(tmp)

	require.NoError(t, os.WriteFile(filepath.Join(tmp, "hostname"), []byte("d2vm-flatten-test"), perm))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "resolv.conf"), []byte("nameserver 8.8.8.8"), perm))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "Dockerfile"), []byte(dockerfile), perm))
	require.NoError(t, docker.Build(ctx, img, "", tmp))
	defer docker.Remove(ctx, img)

	imgTmp := filepath.Join(tmp, "image")

	i, err := NewImage(ctx, img, imgTmp)
	require.NoError(t, err)

	rootfs := filepath.Join(tmp, "rootfs")
	require.NoError(t, i.Flatten(ctx, rootfs))

	b, err := os.ReadFile(filepath.Join(rootfs, "etc", "resolv.conf"))
	require.NoError(t, err)
	assert.Equal(t, "nameserver 8.8.8.8", string(b))

	b, err = os.ReadFile(filepath.Join(rootfs, "etc", "hostname"))
	require.NoError(t, err)
	assert.Equal(t, "d2vm-flatten-test", string(b))

	_, err = os.Stat(filepath.Join(rootfs, "etc", "apk"))
	assert.Error(t, err)

	require.NoError(t, i.Close())
	_, err = os.Stat(imgTmp)
	assert.Error(t, err)
}
