package run

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/svenwiltink/sparsecat"
)

const (
	serverImg = "ubuntu-20.04"
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
		Use:  "hetzner [options] image-path",
		Args: cobra.ExactArgs(1),
		Run:  Hetzner,
	}
)

func init() {
	HetznerCmd.Flags().StringVarP(&hetznerToken, "token", "t", "", "Hetzner Cloud API token")
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
	i, err := ImgInfo(ctx, imgPath)
	if err != nil {
		return err
	}
	if i.Format != "raw" {
		return fmt.Errorf("image format must be raw")
	}
	src, err := os.Open(imgPath)
	if err != nil {
		return err
	}
	defer src.Close()
	// TODO(adphi): check image format
	// TODO(adphi): convert to raw if needed

	c := hcloud.NewClient(hcloud.WithToken(hetznerToken))
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
	_, errs := c.Action.WatchProgress(ctx, sres.Action)
	if err := <-errs; err != nil {
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
	f, err := sftpc.Create("/usr/local/bin/sparsecat")
	if err != nil {
		return err
	}
	if err := sftpc.Chmod("/usr/local/bin/sparsecat", 0755); err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, bytes.NewReader(sparsecatBinary)); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	wses, err := sc.NewSession()
	if err != nil {
		return err
	}
	defer wses.Close()
	logrus.Infof("writing image to /dev/sda")
	done := make(chan struct{})
	pr := newProgressReader(sparsecat.NewEncoder(src))
	wses.Stdin = pr
	go func() {
		tk := time.NewTicker(time.Second)
		last := 0
		for {
			select {
			case <-tk.C:
				b := pr.Progress()
				logrus.Infof("%s / %d%% transfered ( %s/s)", humanize.Bytes(uint64(b)), int(float64(b)/float64(i.ActualSize)*100), humanize.Bytes(uint64(b-last)))
				last = b
			case <-ctx.Done():
				return
			case <-done:
				return
			}
		}
	}()
	if b, err := wses.CombinedOutput("/usr/local/bin/sparsecat -r -disable-sparse-target -of /dev/sda"); err != nil {
		logrus.Fatalf("%v: %s", err, string(b))
	}
	close(done)
	logrus.Infof("rebooting server")
	rbres, _, err := c.Server.Reboot(ctx, sres.Server)
	if err != nil {
		return err
	}
	_, errs = c.Action.WatchProgress(ctx, rbres)
	if err := <-errs; err != nil {
		return err
	}
	logrus.Infof("server created")
	remove = false
	args := []string{"-o", "StrictHostKeyChecking=no"}
	if hetznerSSHKeyPath != "" {
		args = append(args, "-i", hetznerSSHKeyPath)
	}
	args = append(args, fmt.Sprintf("%s@%s", hetznerSSHUser, sres.Server.PublicNet.IPv4.IP.String()))
	makeCmd := func() *exec.Cmd {
		cmd := exec.CommandContext(ctx, "ssh", args...)
		cmd.Stdin = stdin
		cmd.Stderr = stderr
		cmd.Stdout = stdout
		return cmd
	}
	t := time.NewTimer(time.Minute)
	for {
		select {
		case <-t.C:
			return fmt.Errorf("ssh connection timeout")
		case <-ctx.Done():
			return ctx.Err()
		default:
			cmd := makeCmd()
			if err := cmd.Run(); err != nil {
				if strings.Contains(err.Error(), "exit status 255") {
					time.Sleep(time.Second)
					continue
				}
				return err
			} else {
				return nil
			}
		}
	}
}
