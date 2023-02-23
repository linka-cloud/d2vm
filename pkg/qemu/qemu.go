package qemu

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const (
	NetworkingNone    = "none"
	NetworkingUser    = "user"
	NetworkingTap     = "tap"
	NetworkingBridge  = "bridge"
	NetworkingDefault = NetworkingUser
)

var (
	defaultArch  string
	defaultAccel string
)

func init() {
	switch runtime.GOARCH {
	case "arm64":
		defaultArch = "aarch64"
	case "amd64":
		defaultArch = "x86_64"
	case "s390x":
		defaultArch = "s390x"
	}
	switch {
	case runtime.GOARCH == "s390x":
		defaultAccel = "kvm"
	case haveKVM():
		defaultAccel = "kvm:tcg"
	case runtime.GOOS == "darwin":
		defaultAccel = "hvf:tcg"
	}
}

func Run(ctx context.Context, path string, opts ...Option) error {
	config := &config{}

	for _, o := range opts {
		o(config)
	}

	config.path = path

	// Generate UUID, so that /sys/class/dmi/id/product_uuid is populated
	config.uuid = uuid.New()
	// These envvars override the corresponding command line
	// options. So this must remain after the `flags.Parse` above.
	// accel = GetStringValue("LINUXKIT_QEMU_ACCEL", accel, "")

	if config.arch == "" {
		config.arch = defaultArch
	}

	if config.accel == "" {
		config.accel = defaultAccel
	}

	if _, err := os.Stat(config.path); err != nil {
		return err
	}

	if config.cpus == 0 {
		config.cpus = 1
	}

	if config.memory == 0 {
		config.memory = 1024
	}

	for i, d := range config.disks {
		id := ""
		if i != 0 {
			id = strconv.Itoa(i)
		}
		if d.Size != 0 && d.Format == "" {
			d.Format = "qcow2"
		}
		if d.Size != 0 && d.Path == "" {
			d.Path = "disk" + id + ".img"
		}
		if d.Path == "" {
			return fmt.Errorf("disk specified with no size or name")
		}
		config.disks[i] = d
	}

	config.disks = append([]Disk{{Path: config.path}}, config.disks...)

	if config.networking == "" || config.networking == "default" {
		dflt := NetworkingDefault
		config.networking = dflt
	}
	netMode := strings.SplitN(config.networking, ",", 2)

	switch netMode[0] {
	case NetworkingUser:
		config.netdevConfig = "user,id=t0"
	case NetworkingTap:
		if len(netMode) != 2 {
			return fmt.Errorf("Not enough arguments for %q networking mode", NetworkingTap)
		}
		if len(config.publishedPorts) != 0 {
			return fmt.Errorf("Port publishing requires %q networking mode", NetworkingUser)
		}
		config.netdevConfig = fmt.Sprintf("tap,id=t0,ifname=%s,script=no,downscript=no", netMode[1])
	case NetworkingBridge:
		if len(netMode) != 2 {
			return fmt.Errorf("Not enough arguments for %q networking mode", NetworkingBridge)
		}
		if len(config.publishedPorts) != 0 {
			return fmt.Errorf("Port publishing requires %q networking mode", NetworkingUser)
		}
		config.netdevConfig = fmt.Sprintf("bridge,id=t0,br=%s", netMode[1])
	case NetworkingNone:
		if len(config.publishedPorts) != 0 {
			return fmt.Errorf("Port publishing requires %q networking mode", NetworkingUser)
		}
		config.netdevConfig = ""
	default:
		return fmt.Errorf("Invalid networking mode: %s", netMode[0])
	}

	if err := config.discoverBinaries(); err != nil {
		log.Fatal(err)
	}

	return config.runQemuLocal(ctx)
}

func (c *config) runQemuLocal(ctx context.Context) (err error) {
	var args []string
	args, err = c.buildQemuCmdline()
	if err != nil {
		return err
	}

	for _, d := range c.disks {
		// If disk doesn't exist then create one
		if _, err := os.Stat(d.Path); err != nil {
			if os.IsNotExist(err) {
				log.Debugf("Creating new qemu disk [%s] format %s", d.Path, d.Format)
				qemuImgCmd := exec.Command(c.qemuImgPath, "create", "-f", d.Format, d.Path, fmt.Sprintf("%dM", d.Size))
				log.Debugf("%v", qemuImgCmd.Args)
				if err := qemuImgCmd.Run(); err != nil {
					return fmt.Errorf("Error creating disk [%s] format %s:  %s", d.Path, d.Format, err.Error())
				}
			} else {
				return err
			}
		} else {
			log.Infof("Using existing disk [%s] format %s", d.Path, d.Format)
		}
	}

	// Detached mode is only supported in a container.
	if c.detached == true {
		return fmt.Errorf("Detached mode is only supported when running in a container, not locally")
	}

	qemuCmd := exec.CommandContext(ctx, c.qemuBinPath, args...)
	// If verbosity is enabled print out the full path/arguments
	log.Debugf("%v", qemuCmd.Args)

	// If we're not using a separate window then link the execution to stdin/out
	if c.gui == true {
		qemuCmd.Stdin = nil
		qemuCmd.Stdout = nil
		qemuCmd.Stderr = nil
	} else {
		qemuCmd.Stdin = c.stdin
		qemuCmd.Stdout = c.stdout
		qemuCmd.Stderr = c.stderr
	}

	return qemuCmd.Run()
}

func (c *config) buildQemuCmdline() ([]string, error) {
	// Iterate through the flags and build arguments
	var qemuArgs []string
	qemuArgs = append(qemuArgs, "-smp", fmt.Sprintf("%d", c.cpus))
	qemuArgs = append(qemuArgs, "-m", fmt.Sprintf("%d", c.memory))
	qemuArgs = append(qemuArgs, "-uuid", c.uuid.String())

	// Need to specify the vcpu type when running qemu on arm64 platform, for security reason,
	// the vcpu should be "host" instead of other names such as "cortex-a53"...
	if c.arch == "aarch64" {
		if runtime.GOARCH == "arm64" {
			qemuArgs = append(qemuArgs, "-cpu", "host")
		} else {
			qemuArgs = append(qemuArgs, "-cpu", "cortex-a57")
		}
	}

	// goArch is the GOARCH equivalent of config.Arch
	var goArch string
	switch c.arch {
	case "s390x":
		goArch = "s390x"
	case "aarch64":
		goArch = "arm64"
	case "x86_64":
		goArch = "amd64"
	default:
		return nil, fmt.Errorf("%s is an unsupported architecture.", c.arch)
	}

	if goArch != runtime.GOARCH {
		log.Infof("Disable acceleration as %s != %s", c.arch, runtime.GOARCH)
		c.accel = ""
	}

	if c.accel != "" {
		switch c.arch {
		case "s390x":
			qemuArgs = append(qemuArgs, "-machine", fmt.Sprintf("s390-ccw-virtio,accel=%s", c.accel))
		case "aarch64":
			gic := ""
			// VCPU supports less PA bits (36) than requested by the memory map (40)
			highmem := "highmem=off,"
			if runtime.GOOS == "linux" {
				// gic-version=host requires KVM, which implies Linux
				gic = "gic_version=host,"
				highmem = ""
			}
			qemuArgs = append(qemuArgs, "-machine", fmt.Sprintf("virt,%s%saccel=%s", gic, highmem, c.accel))
		default:
			qemuArgs = append(qemuArgs, "-machine", fmt.Sprintf("q35,accel=%s", c.accel))
		}
	} else {
		switch c.arch {
		case "s390x":
			qemuArgs = append(qemuArgs, "-machine", "s390-ccw-virtio")
		case "aarch64":
			qemuArgs = append(qemuArgs, "-machine", "virt")
		default:
			qemuArgs = append(qemuArgs, "-machine", "q35")
		}
	}

	// rng-random does not work on macOS
	// Temporarily disable it until fixed upstream.
	if runtime.GOOS != "darwin" {
		rng := "rng-random,id=rng0"
		if runtime.GOOS == "linux" {
			rng = rng + ",filename=/dev/urandom"
		}
		if c.arch == "s390x" {
			qemuArgs = append(qemuArgs, "-object", rng, "-device", "virtio-rng-ccw,rng=rng0")
		} else {
			qemuArgs = append(qemuArgs, "-object", rng, "-device", "virtio-rng-pci,rng=rng0")
		}
	}

	var lastDisk int
	for i, d := range c.disks {
		index := i
		if d.Format != "" {
			qemuArgs = append(qemuArgs, "-drive", "file="+d.Path+",format="+d.Format+",index="+strconv.Itoa(index)+",media=disk")
		} else {
			qemuArgs = append(qemuArgs, "-drive", "file="+d.Path+",index="+strconv.Itoa(index)+",media=disk")
		}
		lastDisk = index
	}

	// Ensure CDROMs start from at least hdc
	if lastDisk < 2 {
		lastDisk = 2
	}

	if c.netdevConfig != "" {
		mac := generateMAC()
		if c.arch == "s390x" {
			qemuArgs = append(qemuArgs, "-device", "virtio-net-ccw,netdev=t0,mac="+mac.String())
		} else {
			qemuArgs = append(qemuArgs, "-device", "virtio-net-pci,netdev=t0,mac="+mac.String())
		}
		forwardings, err := buildQemuForwardings(c.publishedPorts)
		if err != nil {
			log.Error(err)
		}
		qemuArgs = append(qemuArgs, "-netdev", c.netdevConfig+forwardings)
	} else {
		qemuArgs = append(qemuArgs, "-net", "none")
	}

	if c.gui != true {
		qemuArgs = append(qemuArgs, "-nographic")
	}

	return qemuArgs, nil
}

func (c *config) discoverBinaries() error {
	if c.qemuImgPath != "" {
		return nil
	}

	qemuBinPath := "qemu-system-" + c.arch
	qemuImgPath := "qemu-img"

	var err error
	c.qemuBinPath, err = exec.LookPath(qemuBinPath)
	if err != nil {
		return fmt.Errorf("Unable to find %s within the $PATH", qemuBinPath)
	}

	c.qemuImgPath, err = exec.LookPath(qemuImgPath)
	if err != nil {
		return fmt.Errorf("Unable to find %s within the $PATH", qemuImgPath)
	}

	return nil
}

func buildQemuForwardings(publishedPorts []PublishedPort) (string, error) {
	if len(publishedPorts) == 0 {
		return "", nil
	}
	var forwardings string
	for _, p := range publishedPorts {
		hostPort := p.Host
		guestPort := p.Guest

		forwardings = fmt.Sprintf("%s,hostfwd=%s::%d-:%d", forwardings, p.Protocol, hostPort, guestPort)
	}

	return forwardings, nil
}

func haveKVM() bool {
	_, err := os.Stat("/dev/kvm")
	return !os.IsNotExist(err)
}

func generateMAC() net.HardwareAddr {
	mac := make([]byte, 6)
	n, err := rand.Read(mac)
	if err != nil {
		log.WithError(err).Fatal("failed to generate random mac address")
	}
	if n != 6 {
		log.WithError(err).Fatalf("generated %d bytes for random mac address", n)
	}
	mac[0] &^= 0x01 // Clear multicast bit
	mac[0] |= 0x2   // Set locally administered bit
	return mac
}
