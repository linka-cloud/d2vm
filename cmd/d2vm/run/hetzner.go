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

package run

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/svenwiltink/sparsecat"

	"go.linka.cloud/d2vm/pkg/qemu_img"
)

const (
	hetznerTokenEnv = "HETZNER_TOKEN"
	serverImg       = "ubuntu-20.04"
	vmBlockPath     = "/dev/sda"
	sparsecatPath   = "/usr/local/bin/sparsecat"
)

var (
	hetznerVMType = "cx11"
	hetznerToken  = ""
	// ash-dc1    fsn1-dc14  hel1-dc2   nbg1-dc3
	hetznerDatacenter = "hel1-dc2"
	hetznerServerName = "d2vm"
	hetznerSSHUser    = ""
	hetznerSSHKeyPath = ""
	hetznerRemove     = false

	HetznerCmd = &cobra.Command{
		Use:   "hetzner [options] image-path",
		Short: "Run the virtual machine image on Hetzner Cloud",
		Args:  cobra.ExactArgs(1),
		Run:   Hetzner,
	}
)

func init() {
	HetznerCmd.Flags().StringVar(&hetznerToken, "token", "", "Hetzner Cloud API token [$"+hetznerTokenEnv+"]")
	HetznerCmd.Flags().StringVarP(&hetznerSSHUser, "user", "u", "root", "d2vm image ssh user")
	HetznerCmd.Flags().StringVarP(&hetznerSSHKeyPath, "ssh-key", "i", "", "d2vm image identity key")
	HetznerCmd.Flags().BoolVar(&hetznerRemove, "rm", false, "remove server when done")
	HetznerCmd.Flags().StringVarP(&hetznerServerName, "name", "n", "d2vm", "d2vm server name")
}

func Hetzner(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()
	if err := runHetzner(ctx, args[0], cmd.InOrStdin(), cmd.ErrOrStderr(), cmd.OutOrStdout()); err != nil {
		logrus.Fatal(err)
	}
}

func runHetzner(ctx context.Context, imgPath string, stdin io.Reader, stderr io.Writer, stdout io.Writer) error {
	i, err := qemu_img.Info(ctx, imgPath)
	if err != nil {
		return err
	}
	if i.Format != "raw" {
		logrus.Warnf("image format is %s, expected raw", i.Format)
		rawPath := filepath.Join(os.TempDir(), "d2vm", "run", filepath.Base(imgPath)+".raw")
		if err := os.MkdirAll(filepath.Dir(rawPath), 0755); err != nil {
			return err
		}
		defer os.RemoveAll(rawPath)
		logrus.Infof("converting image to raw: %s", rawPath)
		if err := qemu_img.Convert(ctx, "raw", imgPath, rawPath); err != nil {
			return err
		}
		imgPath = rawPath
		i, err = qemu_img.Info(ctx, imgPath)
		if err != nil {
			return err
		}
	}
	src, err := os.Open(imgPath)
	if err != nil {
		return err
	}
	defer src.Close()

	c := hcloud.NewClient(hcloud.WithToken(GetStringValue(hetznerTokenEnv, hetznerToken, "")))
	st, _, err := c.ServerType.GetByName(ctx, hetznerVMType)
	if err != nil {
		return err
	}
	img, _, err := c.Image.GetByName(ctx, serverImg)
	if err != nil {
		return err
	}
	l, _, err := c.Location.Get(ctx, hetznerDatacenter)
	if err != nil {
		return err
	}
	logrus.Infof("creating server %s", hetznerServerName)
	sres, _, err := c.Server.Create(ctx, hcloud.ServerCreateOpts{
		Name:             hetznerServerName,
		ServerType:       st,
		Image:            img,
		Location:         l,
		StartAfterCreate: hcloud.Bool(false),
	})
	if err != nil {
		return err
	}
	remove := true
	defer func() {
		if !remove && !hetznerRemove {
			return
		}
		logrus.Infof("removing server %s", hetznerServerName)
		// we use context.Background() here because we don't want the request to fail if the context has been cancelled
		if _, err := c.Server.Delete(context.Background(), sres.Server); err != nil {
			logrus.Fatalf("failed to remove server: %v", err)
		}
	}()
	_, errs := c.Action.WatchProgress(ctx, sres.Action)
	if err := <-errs; err != nil {
		return err
	}
	logrus.Infof("server created with ip: %s", sres.Server.PublicNet.IPv4.IP.String())
	logrus.Infof("enabling server rescue mode")
	rres, _, err := c.Server.EnableRescue(ctx, sres.Server, hcloud.ServerEnableRescueOpts{Type: hcloud.ServerRescueTypeLinux64})
	if err != nil {
		return err
	}
	_, errs = c.Action.WatchProgress(ctx, rres.Action)
	if err := <-errs; err != nil {
		return err
	}
	logrus.Infof("powering on server")
	pres, _, err := c.Server.Poweron(ctx, sres.Server)
	if err != nil {
		return err
	}
	_, errs = c.Action.WatchProgress(ctx, pres)
	if err := <-errs; err != nil {
		return err
	}
	logrus.Infof("connecting to server via ssh")
	sc, err := dialSSHWithTimeout(sres.Server.PublicNet.IPv4.IP.String(), "root", rres.RootPassword, time.Minute)
	if err != nil {
		return err
	}
	defer sc.Close()
	logrus.Infof("connection established")
	sftpc, err := sftp.NewClient(sc)
	if err != nil {
		return err
	}
	f, err := sftpc.Create(sparsecatPath)
	logrus.Debugf("creating sparsecat on remote host")
	if err != nil {
		return err
	}
	if err := sftpc.Chmod(sparsecatPath, 0755); err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, bytes.NewReader(sparsecatBinary)); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	serrs := make(chan error, 2)
	go func() {
		serrs <- func() error {
			s, err := sc.NewSession()
			if err != nil {
				return err
			}
			defer s.Close()
			logrus.Infof("installing cloud-guest-utils on rescue server")
			cmd := "apt update && apt install -y cloud-guest-utils"
			logrus.Debugf("$ %s", cmd)
			if b, err := s.CombinedOutput(cmd); err != nil {
				return fmt.Errorf("%v: %s", err, string(b))
			}
			return nil
		}()
	}()
	go func() {
		serrs <- func() error {
			wses, err := sc.NewSession()
			if err != nil {
				return err
			}
			defer wses.Close()
			logrus.Infof("writing image to %s", vmBlockPath)
			done := make(chan struct{})
			defer close(done)
			var r io.Reader
			if runtime.GOOS == "linux" {
				r = sparsecat.NewEncoder(src)
			} else {
				r = src
			}
			pr := newProgressReader(r)
			wses.Stdin = pr
			go func() {
				tk := time.NewTicker(time.Second)
				last := 0
				for {
					select {
					case <-tk.C:
						b := pr.Progress()
						logrus.Infof("%s / %d%% transfered (%s/s)", humanize.Bytes(uint64(b)), int(float64(b)/float64(i.VirtualSize)*100), humanize.Bytes(uint64(b-last)))
						last = b
					case <-ctx.Done():
						logrus.Warnf("context cancelled")
						return
					case <-done:
						logrus.Infof("transfer finished")
						return
					}
				}
			}()
			var cmd string
			if runtime.GOOS == "linux" {
				cmd = fmt.Sprintf("%s -r -disable-sparse-target -of %s", sparsecatPath, vmBlockPath)
			} else {
				cmd = fmt.Sprintf("dd of=%s", vmBlockPath)
			}
			logrus.Debugf("$ %s", cmd)
			if b, err := wses.CombinedOutput(cmd); err != nil {
				return fmt.Errorf("%v: %s", err, string(b))
			} else {
				logrus.Debugf(string(b))
			}
			return nil
		}()
	}()
	for i := 0; i < 2; i++ {
		select {
		case err := <-serrs:
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	gses, err := sc.NewSession()
	if err != nil {
		return err
	}
	defer gses.Close()
	logrus.Infof("resizing disk partition")
	cmd := fmt.Sprintf("growpart %s 1", vmBlockPath)
	logrus.Debugf("$ %s", cmd)
	if b, err := gses.CombinedOutput(cmd); err != nil {
		return fmt.Errorf("%v: %s", err, string(b))
	} else {
		logrus.Debugf(string(b))
	}
	cses, err := sc.NewSession()
	if err != nil {
		return err
	}
	defer cses.Close()
	logrus.Infof("checking disk partition")
	cmd = fmt.Sprintf("e2fsck -yf %s1", vmBlockPath)
	logrus.Debugf("$ %s", cmd)
	if b, err := cses.CombinedOutput(cmd); err != nil {
		return fmt.Errorf("%v: %s", err, string(b))
	} else {
		logrus.Debugf(string(b))
	}
	eses, err := sc.NewSession()
	if err != nil {
		return err
	}
	defer eses.Close()
	logrus.Infof("extending partition file system")
	cmd = fmt.Sprintf("resize2fs %s1", vmBlockPath)
	logrus.Debugf("$ %s", cmd)
	if b, err := eses.CombinedOutput(cmd); err != nil {
		return fmt.Errorf("%v: %s", err, string(b))
	}
	logrus.Infof("rebooting server")
	rbres, _, err := c.Server.Reboot(ctx, sres.Server)
	if err != nil {
		return err
	}
	_, errs = c.Action.WatchProgress(ctx, rbres)
	if err := <-errs; err != nil {
		return err
	}
	remove = false
	logrus.Infof("waiting for server to be ready")
	t := time.NewTimer(time.Minute)
wait:
	for {
		select {
		case <-t.C:
			return fmt.Errorf("ssh connection timeout")
		case <-ctx.Done():
			return ctx.Err()
		default:
			var d net.Dialer
			conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:22", sres.Server.PublicNet.IPv4.IP.String()))
			if err == nil {
				conn.Close()
				break wait
			}
			time.Sleep(time.Second)
		}
	}
	logrus.Infof("server ready")
	args := []string{"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null"}
	if hetznerSSHKeyPath != "" {
		args = append(args, "-i", hetznerSSHKeyPath)
	}
	args = append(args, fmt.Sprintf("%s@%s", hetznerSSHUser, sres.Server.PublicNet.IPv4.IP.String()))
	logrus.Debugf("$ ssh %s", strings.Join(args, " "))
	sshCmd := exec.CommandContext(ctx, "ssh", args...)
	sshCmd.Stdin = stdin
	sshCmd.Stderr = stderr
	sshCmd.Stdout = stdout
	if err := sshCmd.Run(); err != nil {
		return err
	}
	return nil
}
