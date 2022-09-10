package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.linka.cloud/console"

	exec2 "go.linka.cloud/d2vm/pkg/exec"
)

var (
	VboxCmd = &cobra.Command{
		Use:   "vbox [options] image-path",
		Short: "Run the virtual machine image with Virtualbox",
		Args:  cobra.ExactArgs(1),
		Run:   Vbox,
	}

	vboxmanageFlag string
	name           string
	networks       VBNetworks
)

func init() {
	flags := VboxCmd.Flags()
	// Display flags
	flags.Bool("gui", false, "Show the VM GUI")

	// vbox options
	flags.StringVar(&vboxmanageFlag, "vboxmanage", "VBoxManage", "VBoxManage binary to use")
	flags.StringVar(&name, "name", "d2vm", "Name of the Virtualbox VM")

	// Paths and settings for disks
	flags.Var(&disks, "disk", "Disk config, may be repeated. [file=]path[,size=1G][,format=raw]")

	// VM configuration
	flags.Uint("cpus", 1, "Number of CPUs")
	flags.Uint("mem", 1024, "Amount of memory in MB")

	// networking
	flags.Var(&networks, "networking", "Network config, may be repeated. [type=](null|nat|bridged|intnet|hostonly|generic|natnetwork[<devicename>])[,[bridge|host]adapter=<interface>]")

	if runtime.GOOS == "windows" {
		log.Fatalf("TODO: Windows is not yet supported")
	}
}

func Vbox(cmd *cobra.Command, args []string) {
	path := args[0]
	if err := vbox(cmd.Context(), path); err != nil {
		logrus.Fatal(err)
	}
}

func vbox(ctx context.Context, path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	vboxmanage, err := exec.LookPath(vboxmanageFlag)
	if err != nil {
		return fmt.Errorf("Cannot find management binary %s: %v", vboxmanageFlag, err)
	}
	i, err := ImgInfo(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to get image info: %v", err)
	}
	if i.Format != "vdi" {
		logrus.Warnf("image format is %s, expected vdi", i.Format)
		vdi := filepath.Join(os.TempDir(), "d2vm", "run", filepath.Base(path)+".vdi")
		if err := os.MkdirAll(filepath.Dir(vdi), 0755); err != nil {
			return err
		}
		defer os.RemoveAll(vdi)
		logrus.Infof("converting image to raw: %s", vdi)
		if err := exec2.Run(ctx, "qemu-img", "convert", "-O", "vdi", path, vdi); err != nil {
			return err
		}
		path = vdi
	}

	// remove machine in case it already exists
	cleanup(vboxmanage, name)

	_, out, err := manage(vboxmanage, "createvm", "--name", name, "--register")
	if err != nil {
		return fmt.Errorf("createvm error: %v\n%s", err, out)
	}

	_, out, err = manage(vboxmanage, "modifyvm", name, "--acpi", "on")
	if err != nil {
		return fmt.Errorf("modifyvm --acpi error: %v\n%s", err, out)
	}

	_, out, err = manage(vboxmanage, "modifyvm", name, "--memory", fmt.Sprintf("%d", mem))
	if err != nil {
		return fmt.Errorf("modifyvm --memory error: %v\n%s", err, out)
	}

	_, out, err = manage(vboxmanage, "modifyvm", name, "--cpus", fmt.Sprintf("%d", cpus))
	if err != nil {
		return fmt.Errorf("modifyvm --cpus error: %v\n%s", err, out)
	}

	_, out, err = manage(vboxmanage, "modifyvm", name, "--firmware", "bios")
	if err != nil {
		return fmt.Errorf("modifyvm --firmware error: %v\n%s", err, out)
	}

	// set up serial console
	_, out, err = manage(vboxmanage, "modifyvm", name, "--uart1", "0x3F8", "4")
	if err != nil {
		return fmt.Errorf("modifyvm --uart error: %v\n%s", err, out)
	}

	consolePath := filepath.Join(os.TempDir(), "d2vm-vb", name, "console")
	if err := os.MkdirAll(filepath.Dir(consolePath), os.ModePerm); err != nil {
		return fmt.Errorf("mkir %s: %v", consolePath, err)
	}
	if runtime.GOOS != "windows" {
		consolePath, err = filepath.Abs(consolePath)
		if err != nil {
			return fmt.Errorf("Bad path: %v", err)
		}
	} else {
		// TODO use a named pipe on Windows
	}

	_, out, err = manage(vboxmanage, "modifyvm", name, "--uartmode1", "client", consolePath)
	if err != nil {
		return fmt.Errorf("modifyvm --uartmode error: %v\n%s", err, out)
	}

	_, out, err = manage(vboxmanage, "storagectl", name, "--name", "IDE Controller", "--add", "ide")
	if err != nil {
		return fmt.Errorf("storagectl error: %v\n%s", err, out)
	}

	_, out, err = manage(vboxmanage, "storageattach", name, "--storagectl", "IDE Controller", "--port", "1", "--device", "0", "--type", "hdd", "--medium", path)
	if err != nil {
		return fmt.Errorf("storageattach error: %v\n%s", err, out)
	}
	_, out, err = manage(vboxmanage, "modifyvm", name, "--boot1", "disk")
	if err != nil {
		return fmt.Errorf("modifyvm --boot error: %v\n%s", err, out)
	}

	if len(disks) > 0 {
		_, out, err = manage(vboxmanage, "storagectl", name, "--name", "SATA", "--add", "sata")
		if err != nil {
			return fmt.Errorf("storagectl error: %v\n%s", err, out)
		}
	}

	for i, d := range disks {
		id := strconv.Itoa(i)
		if d.Size != 0 && d.Format == "" {
			d.Format = "raw"
		}
		if d.Format != "raw" && d.Path == "" {
			return fmt.Errorf("vbox currently can only create raw disks")
		}
		if d.Path == "" && d.Size == 0 {
			return fmt.Errorf("please specify an existing disk file or a size")
		}
		if d.Path == "" {
			d.Path = "disk" + id + ".img"
			if err := os.Truncate(d.Path, int64(d.Size)*int64(1048576)); err != nil {
				return fmt.Errorf("Cannot create disk: %v", err)
			}
		}
		_, out, err = manage(vboxmanage, "storageattach", name, "--storagectl", "SATA", "--port", "0", "--device", id, "--type", "hdd", "--medium", d.Path)
		if err != nil {
			return fmt.Errorf("storageattach error: %v\n%s", err, out)
		}
	}

	for i, d := range networks {
		nic := i + 1
		_, out, err = manage(vboxmanage, "modifyvm", name, fmt.Sprintf("--nictype%d", nic), "virtio")
		if err != nil {
			return fmt.Errorf("modifyvm --nictype error: %v\n%s", err, out)
		}

		_, out, err = manage(vboxmanage, "modifyvm", name, fmt.Sprintf("--nic%d", nic), d.Type)
		if err != nil {
			return fmt.Errorf("modifyvm --nic error: %v\n%s", err, out)
		}
		if d.Type == "hostonly" {
			_, out, err = manage(vboxmanage, "modifyvm", name, fmt.Sprintf("--hostonlyadapter%d", nic), d.Adapter)
			if err != nil {
				return fmt.Errorf("modifyvm --hostonlyadapter error: %v\n%s", err, out)
			}
		} else if d.Type == "bridged" {
			_, out, err = manage(vboxmanage, "modifyvm", name, fmt.Sprintf("--bridgeadapter%d", nic), d.Adapter)
			if err != nil {
				return fmt.Errorf("modifyvm --bridgeadapter error: %v\n%s", err, out)
			}
		}

		_, out, err = manage(vboxmanage, "modifyvm", name, fmt.Sprintf("--cableconnected%d", nic), "on")
		if err != nil {
			return fmt.Errorf("modifyvm --cableconnected error: %v\n%s", err, out)
		}
	}

	// create socket
	_ = os.Remove(consolePath)
	ln, err := net.Listen("unix", consolePath)
	if err != nil {
		return fmt.Errorf("Cannot listen on console socket %s: %v", consolePath, err)
	}
	defer ln.Close()

	var vmType string
	if enableGUI {
		vmType = "gui"
	} else {
		vmType = "headless"
	}

	term := console.Current()
	ws, err := term.Size()
	if err != nil {
		return fmt.Errorf("get term size: %v", err)
	}

	_, out, err = manage(vboxmanage, "startvm", name, "--type", vmType)
	if err != nil {
		return fmt.Errorf("startvm error: %v\n%s", err, out)
	}
	defer cleanup(vboxmanage, name)

	if err := term.Resize(ws); err != nil && !errors.Is(err, console.ErrUnsupported) {
		return fmt.Errorf("resize term: %v", err)
	}
	if err := term.SetRaw(); err != nil {
		return fmt.Errorf("set raw term: %v", err)
	}
	defer func() {
		if err := term.Reset(); err != nil {
			log.Errorf("failed to reset term: %v", err)
		}
	}()

	socket, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("Accept error: %v", err)
	}
	defer socket.Close()
	errs := make(chan error, 2)
	go func() {
		_, err := io.Copy(socket, term)
		errs <- err
	}()
	go func() {
		_, err := io.Copy(term, socket)
		errs <- err
	}()
	return <-errs
}

func cleanup(vboxmanage string, name string) {
	if _, _, err := manage(vboxmanage, "controlvm", name, "poweroff"); err != nil {
		log.Errorf("controlvm poweroff error: %v", err)
	}
	_, out, err := manage(vboxmanage, "storageattach", name, "--storagectl", "IDE Controller", "--port", "1", "--device", "0", "--type", "hdd", "--medium", "emptydrive")
	if err != nil {
		log.Errorf("storageattach error: %v\n%s", err, out)
	}
	for i := range disks {
		id := strconv.Itoa(i)
		_, out, err := manage(vboxmanage, "storageattach", name, "--storagectl", "SATA", "--port", "0", "--device", id, "--type", "hdd", "--medium", "emptydrive")
		if err != nil {
			log.Errorf("storageattach error: %v\n%s", err, out)
		}
	}
	if _, out, err = manage(vboxmanage, "unregistervm", name, "--delete"); err != nil {
		log.Errorf("unregistervm error: %v\n%s", err, out)
	}
}

func manage(vboxmanage string, args ...string) (string, string, error) {
	log.Debugf("$ %s %s", vboxmanage, strings.Join(args, " "))
	cmd := exec.Command(vboxmanage, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = io.MultiWriter(&stdout, logrus.StandardLogger().WriterLevel(logrus.DebugLevel))
	cmd.Stderr = io.MultiWriter(&stderr, logrus.StandardLogger().WriterLevel(logrus.DebugLevel))
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// VBNetwork is the config for a Virtual Box network
type VBNetwork struct {
	Type    string
	Adapter string
}

// VBNetworks is the type for a list of VBNetwork
type VBNetworks []VBNetwork

func (l *VBNetworks) String() string {
	return fmt.Sprint(*l)
}

func (l *VBNetworks) Type() string {
	return "vbnetworks"
}

// Set is used by flag to configure value from CLI
func (l *VBNetworks) Set(value string) error {
	d := VBNetwork{}
	s := strings.Split(value, ",")
	for _, p := range s {
		c := strings.SplitN(p, "=", 2)
		switch len(c) {
		case 1:
			d.Type = c[0]
		case 2:
			switch c[0] {
			case "type":
				d.Type = c[1]
			case "adapter", "bridgeadapter", "hostadapter":
				d.Adapter = c[1]
			default:
				return fmt.Errorf("Unknown network config: %s", c[0])
			}
		}
	}
	*l = append(*l, d)
	return nil
}
