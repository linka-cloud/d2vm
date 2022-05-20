package run

import (
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
	"github.com/spf13/cobra"
)

const (
	qemuNetworkingNone    string = "none"
	qemuNetworkingUser           = "user"
	qemuNetworkingTap            = "tap"
	qemuNetworkingBridge         = "bridge"
	qemuNetworkingDefault        = qemuNetworkingUser
)

var (
	defaultArch  string
	defaultAccel string
	enableGUI    *bool
	disks        Disks
	data         *string
	accel        *string
	arch         *string
	cpus         *uint
	mem          *uint
	qemuCmd      *string
	qemuDetached *bool
	networking   *string
	publishFlags MultipleFlag
	deviceFlags  MultipleFlag
	usbEnabled   *bool

	QemuCmd = &cobra.Command{
		Use:  "qemu [options] [image-path]",
		Args: cobra.ExactArgs(1),
		Run:  Qemu,
	}
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
	flags := QemuCmd.Flags()
	// flags.Usage = func() {
	// 	fmt.Printf("Options:")
	// 	flags.PrintDefaults()
	// 	fmt.Printf("")
	// 	fmt.Printf("If not running as root note that '--networking bridge,br0' requires a")
	// 	fmt.Printf("setuid network helper and appropriate host configuration, see")
	// 	fmt.Printf("https://wiki.qemu.org/Features/HelperNetworking")
	// }
	enableGUI = flags.Bool("gui", false, "Set qemu to use video output instead of stdio")

	// Paths and settings for disks
	flags.Var(&disks, "disk", "Disk config, may be repeated. [file=]path[,size=1G][,format=qcow2]")
	data = flags.String("data", "", "String of metadata to pass to VM; error to specify both -data and -data-file")

	// VM configuration
	accel = flags.String("accel", defaultAccel, "Choose acceleration mode. Use 'tcg' to disable it.")
	arch = flags.String("arch", defaultArch, "Type of architecture to use, e.g. x86_64, aarch64, s390x")
	cpus = flags.Uint("cpus", 1, "Number of CPUs")
	mem = flags.Uint("mem", 1024, "Amount of memory in MB")

	// Backend configuration
	qemuCmd = flags.String("qemu", "", "Path to the qemu binary (otherwise look in $PATH)")
	qemuDetached = flags.Bool("detached", false, "Set qemu container to run in the background")

	// Networking
	networking = flags.String("networking", qemuNetworkingDefault, "Networking mode. Valid options are 'default', 'user', 'bridge[,name]', tap[,name] and 'none'. 'user' uses QEMUs userspace networking. 'bridge' connects to a preexisting bridge. 'tap' uses a prexisting tap device. 'none' disables networking.`")

	flags.Var(&publishFlags, "publish", "Publish a vm's port(s) to the host (default [])")

	// USB devices
	usbEnabled = flags.Bool("usb", false, "Enable USB controller")

	flags.Var(&deviceFlags, "device", "Add USB host device(s). Format driver[,prop=value][,...] -- add device, like -device on the qemu command line.")

}

func Qemu(cmd *cobra.Command, args []string) {

	// Generate UUID, so that /sys/class/dmi/id/product_uuid is populated
	vmUUID := uuid.New()
	// These envvars override the corresponding command line
	// options. So this must remain after the `flags.Parse` above.
	*accel = GetStringValue("LINUXKIT_QEMU_ACCEL", *accel, "")

	path := args[0]

	if _, err := os.Stat(path); err != nil {
		log.Fatal(err)
	}

	for i, d := range disks {
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
			log.Fatalf("disk specified with no size or name")
		}
		disks[i] = d
	}

	disks = append(Disks{DiskConfig{Path: path}}, disks...)

	if *networking == "" || *networking == "default" {
		dflt := qemuNetworkingDefault
		networking = &dflt
	}
	netMode := strings.SplitN(*networking, ",", 2)

	var netdevConfig string
	switch netMode[0] {
	case qemuNetworkingUser:
		netdevConfig = "user,id=t0"
	case qemuNetworkingTap:
		if len(netMode) != 2 {
			log.Fatalf("Not enough arguments for %q networking mode", qemuNetworkingTap)
		}
		if len(publishFlags) != 0 {
			log.Fatalf("Port publishing requires %q networking mode", qemuNetworkingUser)
		}
		netdevConfig = fmt.Sprintf("tap,id=t0,ifname=%s,script=no,downscript=no", netMode[1])
	case qemuNetworkingBridge:
		if len(netMode) != 2 {
			log.Fatalf("Not enough arguments for %q networking mode", qemuNetworkingBridge)
		}
		if len(publishFlags) != 0 {
			log.Fatalf("Port publishing requires %q networking mode", qemuNetworkingUser)
		}
		netdevConfig = fmt.Sprintf("bridge,id=t0,br=%s", netMode[1])
	case qemuNetworkingNone:
		if len(publishFlags) != 0 {
			log.Fatalf("Port publishing requires %q networking mode", qemuNetworkingUser)
		}
		netdevConfig = ""
	default:
		log.Fatalf("Invalid networking mode: %s", netMode[0])
	}

	config := QemuConfig{
		Path:           path,
		GUI:            *enableGUI,
		Disks:          disks,
		Arch:           *arch,
		CPUs:           *cpus,
		Memory:         *mem,
		Accel:          *accel,
		Detached:       *qemuDetached,
		QemuBinPath:    *qemuCmd,
		PublishedPorts: publishFlags,
		NetdevConfig:   netdevConfig,
		UUID:           vmUUID,
		USB:            *usbEnabled,
		Devices:        deviceFlags,
	}

	config, err := discoverBinaries(config)
	if err != nil {
		log.Fatal(err)
	}

	if err = runQemuLocal(config); err != nil {
		log.Fatal(err.Error())
	}
}

func runQemuLocal(config QemuConfig) error {
	var args []string
	config, args = buildQemuCmdline(config)

	for _, d := range config.Disks {
		// If disk doesn't exist then create one
		if _, err := os.Stat(d.Path); err != nil {
			if os.IsNotExist(err) {
				log.Debugf("Creating new qemu disk [%s] format %s", d.Path, d.Format)
				qemuImgCmd := exec.Command(config.QemuImgPath, "create", "-f", d.Format, d.Path, fmt.Sprintf("%dM", d.Size))
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
	if config.Detached == true {
		return fmt.Errorf("Detached mode is only supported when running in a container, not locally")
	}

	qemuCmd := exec.Command(config.QemuBinPath, args...)
	// If verbosity is enabled print out the full path/arguments
	log.Debugf("%v", qemuCmd.Args)

	// If we're not using a separate window then link the execution to stdin/out
	if config.GUI != true {
		qemuCmd.Stdin = os.Stdin
		qemuCmd.Stdout = os.Stdout
		qemuCmd.Stderr = os.Stderr
	}

	return qemuCmd.Run()
}

func buildQemuCmdline(config QemuConfig) (QemuConfig, []string) {
	// Iterate through the flags and build arguments
	var qemuArgs []string
	qemuArgs = append(qemuArgs, "-smp", fmt.Sprintf("%d", config.CPUs))
	qemuArgs = append(qemuArgs, "-m", fmt.Sprintf("%d", config.Memory))
	qemuArgs = append(qemuArgs, "-uuid", config.UUID.String())

	// Need to specify the vcpu type when running qemu on arm64 platform, for security reason,
	// the vcpu should be "host" instead of other names such as "cortex-a53"...
	if config.Arch == "aarch64" {
		if runtime.GOARCH == "arm64" {
			qemuArgs = append(qemuArgs, "-cpu", "host")
		} else {
			qemuArgs = append(qemuArgs, "-cpu", "cortex-a57")
		}
	}

	// goArch is the GOARCH equivalent of config.Arch
	var goArch string
	switch config.Arch {
	case "s390x":
		goArch = "s390x"
	case "aarch64":
		goArch = "arm64"
	case "x86_64":
		goArch = "amd64"
	default:
		log.Fatalf("%s is an unsupported architecture.", config.Arch)
	}

	if goArch != runtime.GOARCH {
		log.Infof("Disable acceleration as %s != %s", config.Arch, runtime.GOARCH)
		config.Accel = ""
	}

	if config.Accel != "" {
		switch config.Arch {
		case "s390x":
			qemuArgs = append(qemuArgs, "-machine", fmt.Sprintf("s390-ccw-virtio,accel=%s", config.Accel))
		case "aarch64":
			gic := ""
			// VCPU supports less PA bits (36) than requested by the memory map (40)
			highmem := "highmem=off,"
			if runtime.GOOS == "linux" {
				// gic-version=host requires KVM, which implies Linux
				gic = "gic_version=host,"
				highmem = ""
			}
			qemuArgs = append(qemuArgs, "-machine", fmt.Sprintf("virt,%s%saccel=%s", gic, highmem, config.Accel))
		default:
			qemuArgs = append(qemuArgs, "-machine", fmt.Sprintf("q35,accel=%s", config.Accel))
		}
	} else {
		switch config.Arch {
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
		if config.Arch == "s390x" {
			qemuArgs = append(qemuArgs, "-object", rng, "-device", "virtio-rng-ccw,rng=rng0")
		} else {
			qemuArgs = append(qemuArgs, "-object", rng, "-device", "virtio-rng-pci,rng=rng0")
		}
	}

	var lastDisk int
	for i, d := range config.Disks {
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

	if config.NetdevConfig == "" {
		qemuArgs = append(qemuArgs, "-net", "none")
	} else {
		mac := generateMAC()
		if config.Arch == "s390x" {
			qemuArgs = append(qemuArgs, "-device", "virtio-net-ccw,netdev=t0,mac="+mac.String())
		} else {
			qemuArgs = append(qemuArgs, "-device", "virtio-net-pci,netdev=t0,mac="+mac.String())
		}
		forwardings, err := buildQemuForwardings(config.PublishedPorts)
		if err != nil {
			log.Error(err)
		}
		qemuArgs = append(qemuArgs, "-netdev", config.NetdevConfig+forwardings)
	}

	if config.GUI != true {
		qemuArgs = append(qemuArgs, "-nographic")
	}

	if config.USB == true {
		qemuArgs = append(qemuArgs, "-usb")
	}
	for _, d := range config.Devices {
		qemuArgs = append(qemuArgs, "-device", d)
	}

	return config, qemuArgs
}

func discoverBinaries(config QemuConfig) (QemuConfig, error) {
	if config.QemuImgPath != "" {
		return config, nil
	}

	qemuBinPath := "qemu-system-" + config.Arch
	qemuImgPath := "qemu-img"

	var err error
	config.QemuBinPath, err = exec.LookPath(qemuBinPath)
	if err != nil {
		return config, fmt.Errorf("Unable to find %s within the $PATH", qemuBinPath)
	}

	config.QemuImgPath, err = exec.LookPath(qemuImgPath)
	if err != nil {
		return config, fmt.Errorf("Unable to find %s within the $PATH", qemuImgPath)
	}

	return config, nil
}

func buildQemuForwardings(publishFlags MultipleFlag) (string, error) {
	if len(publishFlags) == 0 {
		return "", nil
	}
	var forwardings string
	for _, publish := range publishFlags {
		p, err := NewPublishedPort(publish)
		if err != nil {
			return "", err
		}

		hostPort := p.Host
		guestPort := p.Guest

		forwardings = fmt.Sprintf("%s,hostfwd=%s::%d-:%d", forwardings, p.Protocol, hostPort, guestPort)
	}

	return forwardings, nil
}

func buildDockerForwardings(publishedPorts []string) ([]string, error) {
	pmap := []string{}
	for _, port := range publishedPorts {
		s, err := NewPublishedPort(port)
		if err != nil {
			return nil, err
		}
		pmap = append(pmap, "-p", fmt.Sprintf("%d:%d/%s", s.Host, s.Guest, s.Protocol))
	}
	return pmap, nil
}

// QemuConfig contains the config for Qemu
type QemuConfig struct {
	Path           string
	GUI            bool
	Disks          Disks
	FWPath         string
	Arch           string
	CPUs           uint
	Memory         uint
	Accel          string
	Detached       bool
	QemuBinPath    string
	QemuImgPath    string
	PublishedPorts []string
	NetdevConfig   string
	UUID           uuid.UUID
	USB            bool
	Devices        []string
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
	return net.HardwareAddr(mac)
}
