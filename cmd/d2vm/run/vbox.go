package run

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/containerd/console"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	VboxCmd = &cobra.Command{
		Use:  "vbox [options] image-path",
		Args: cobra.ExactArgs(1),
		Run:  Vbox,
	}

	vboxmanageFlag *string
	vmName         *string
	networks       VBNetworks
)

func init() {
	flags := VboxCmd.Flags()
	// Display flags
	enableGUI = flags.Bool("gui", false, "Show the VM GUI")

	// vbox options
	vboxmanageFlag = flags.String("vboxmanage", "VBoxManage", "VBoxManage binary to use")
	vmName = flags.String("name", "", "Name of the Virtualbox VM")

	// Paths and settings for disks
	flags.Var(&disks, "disk", "Disk config, may be repeated. [file=]path[,size=1G][,format=raw]")

	// VM configuration
	cpus = flags.Uint("cpus", 1, "Number of CPUs")
	mem = flags.Uint("mem", 1024, "Amount of memory in MB")

	// networking
	flags.Var(&networks, "networking", "Network config, may be repeated. [type=](null|nat|bridged|intnet|hostonly|generic|natnetwork[<devicename>])[,[bridge|host]adapter=<interface>]")

	if runtime.GOOS == "windows" {
		log.Fatalf("TODO: Windows is not yet supported")
	}
}

func Vbox(cmd *cobra.Command, args []string) {
	path := args[0]

	vboxmanage, err := exec.LookPath(*vboxmanageFlag)
	if err != nil {
		log.Fatalf("Cannot find management binary %s: %v", *vboxmanageFlag, err)
	}

	name := *vmName
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	// remove machine in case it already exists
	cleanup(vboxmanage, name)

	_, out, err := manage(vboxmanage, "createvm", "--name", name, "--register")
	if err != nil {
		log.Fatalf("createvm error: %v\n%s", err, out)
	}

	_, out, err = manage(vboxmanage, "modifyvm", name, "--acpi", "on")
	if err != nil {
		log.Fatalf("modifyvm --acpi error: %v\n%s", err, out)
	}

	_, out, err = manage(vboxmanage, "modifyvm", name, "--memory", fmt.Sprintf("%d", *mem))
	if err != nil {
		log.Fatalf("modifyvm --memory error: %v\n%s", err, out)
	}

	_, out, err = manage(vboxmanage, "modifyvm", name, "--cpus", fmt.Sprintf("%d", *cpus))
	if err != nil {
		log.Fatalf("modifyvm --cpus error: %v\n%s", err, out)
	}

	_, out, err = manage(vboxmanage, "modifyvm", name, "--firmware", "bios")
	if err != nil {
		log.Fatalf("modifyvm --firmware error: %v\n%s", err, out)
	}

	// set up serial console
	_, out, err = manage(vboxmanage, "modifyvm", name, "--uart1", "0x3F8", "4")
	if err != nil {
		log.Fatalf("modifyvm --uart error: %v\n%s", err, out)
	}

	consolePath := filepath.Join(os.TempDir(), "d2vm-vb", name, "console")
	if err := os.MkdirAll(filepath.Dir(consolePath), os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		consolePath, err = filepath.Abs(consolePath)
		if err != nil {
			log.Fatalf("Bad path: %v", err)
		}
	} else {
		// TODO use a named pipe on Windows
	}

	_, out, err = manage(vboxmanage, "modifyvm", name, "--uartmode1", "client", consolePath)
	if err != nil {
		log.Fatalf("modifyvm --uartmode error: %v\n%s", err, out)
	}

	_, out, err = manage(vboxmanage, "storagectl", name, "--name", "IDE Controller", "--add", "ide")
	if err != nil {
		log.Fatalf("storagectl error: %v\n%s", err, out)
	}

	_, out, err = manage(vboxmanage, "storageattach", name, "--storagectl", "IDE Controller", "--port", "1", "--device", "0", "--type", "hdd", "--medium", path)
	if err != nil {
		log.Fatalf("storageattach error: %v\n%s", err, out)
	}
	_, out, err = manage(vboxmanage, "modifyvm", name, "--boot1", "disk")
	if err != nil {
		log.Fatalf("modifyvm --boot error: %v\n%s", err, out)
	}

	if len(disks) > 0 {
		_, out, err = manage(vboxmanage, "storagectl", name, "--name", "SATA", "--add", "sata")
		if err != nil {
			log.Fatalf("storagectl error: %v\n%s", err, out)
		}
	}

	for i, d := range disks {
		id := strconv.Itoa(i)
		if d.Size != 0 && d.Format == "" {
			d.Format = "raw"
		}
		if d.Format != "raw" && d.Path == "" {
			log.Fatal("vbox currently can only create raw disks")
		}
		if d.Path == "" && d.Size == 0 {
			log.Fatal("please specify an existing disk file or a size")
		}
		if d.Path == "" {
			d.Path = "disk" + id + ".img"
			if err := os.Truncate(d.Path, int64(d.Size)*int64(1048576)); err != nil {
				log.Fatalf("Cannot create disk: %v", err)
			}
		}
		_, out, err = manage(vboxmanage, "storageattach", name, "--storagectl", "SATA", "--port", "0", "--device", id, "--type", "hdd", "--medium", d.Path)
		if err != nil {
			log.Fatalf("storageattach error: %v\n%s", err, out)
		}
	}

	for i, d := range networks {
		nic := i + 1
		_, out, err = manage(vboxmanage, "modifyvm", name, fmt.Sprintf("--nictype%d", nic), "virtio")
		if err != nil {
			log.Fatalf("modifyvm --nictype error: %v\n%s", err, out)
		}

		_, out, err = manage(vboxmanage, "modifyvm", name, fmt.Sprintf("--nic%d", nic), d.Type)
		if err != nil {
			log.Fatalf("modifyvm --nic error: %v\n%s", err, out)
		}
		if d.Type == "hostonly" {
			_, out, err = manage(vboxmanage, "modifyvm", name, fmt.Sprintf("--hostonlyadapter%d", nic), d.Adapter)
			if err != nil {
				log.Fatalf("modifyvm --hostonlyadapter error: %v\n%s", err, out)
			}
		} else if d.Type == "bridged" {
			_, out, err = manage(vboxmanage, "modifyvm", name, fmt.Sprintf("--bridgeadapter%d", nic), d.Adapter)
			if err != nil {
				log.Fatalf("modifyvm --bridgeadapter error: %v\n%s", err, out)
			}
		}

		_, out, err = manage(vboxmanage, "modifyvm", name, fmt.Sprintf("--cableconnected%d", nic), "on")
		if err != nil {
			log.Fatalf("modifyvm --cableconnected error: %v\n%s", err, out)
		}
	}

	// create socket
	_ = os.Remove(consolePath)
	ln, err := net.Listen("unix", consolePath)
	if err != nil {
		log.Fatalf("Cannot listen on console socket %s: %v", consolePath, err)
	}

	var vmType string
	if *enableGUI {
		vmType = "gui"
	} else {
		vmType = "headless"
	}

	term := console.Current()
	ws, err := term.Size()
	if err != nil {
		log.Fatal(err)
	}
	if err := term.Resize(ws); err != nil {
		log.Fatal(err)
	}
	if err := term.SetRaw(); err != nil {
		log.Fatal(err)
	}
	defer term.Close()

	_, out, err = manage(vboxmanage, "startvm", name, "--type", vmType)
	if err != nil {
		log.Fatalf("startvm error: %v\n%s", err, out)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		cleanup(vboxmanage, name)
		os.Exit(1)
	}()

	socket, err := ln.Accept()
	if err != nil {
		log.Fatalf("Accept error: %v", err)
	}

	go func() {
		if _, err := io.Copy(socket, term); err != nil {
			cleanup(vboxmanage, name)
			log.Fatalf("Copy error: %v", err)
		}
		cleanup(vboxmanage, name)
		os.Exit(0)
	}()
	go func() {
		if _, err := io.Copy(term, socket); err != nil {
			cleanup(vboxmanage, name)
			log.Fatalf("Copy error: %v", err)
		}
		cleanup(vboxmanage, name)
		os.Exit(0)
	}()
	// wait forever
	select {}
}

func cleanup(vboxmanage string, name string) {
	if _, _, err := manage(vboxmanage, "controlvm", name, "poweroff"); err != nil {
		return
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
	cmd := exec.Command(vboxmanage, args...)
	log.Debugf("[VBOX]: %s %s", vboxmanage, strings.Join(args, " "))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
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
