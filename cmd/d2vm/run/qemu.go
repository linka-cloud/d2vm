package run

import (
	"os"
	"runtime"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"go.linka.cloud/d2vm/pkg/qemu"
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
	enableGUI    bool
	disks        Disks
	data         string
	accel        string
	arch         string
	cpus         uint
	mem          uint
	bios         string
	qemuCmd      string
	qemuDetached bool
	networking   string
	publishFlags MultipleFlag

	QemuCmd = &cobra.Command{
		Use:   "qemu [options] [image-path]",
		Short: "Run the virtual machine image with qemu",
		Args:  cobra.ExactArgs(1),
		Run:   Qemu,
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

	flags.BoolVar(&enableGUI, "gui", false, "Set qemu to use video output instead of stdio")

	// Paths and settings for disks
	flags.Var(&disks, "disk", "Disk config, may be repeated. [file=]path[,size=1G][,format=qcow2]")

	// VM configuration
	flags.StringVar(&accel, "accel", defaultAccel, "Choose acceleration mode. Use 'tcg' to disable it.")
	flags.StringVar(&arch, "arch", defaultArch, "Type of architecture to use, e.g. x86_64, aarch64, s390x")
	flags.UintVar(&cpus, "cpus", 1, "Number of CPUs")
	flags.UintVar(&mem, "mem", 1024, "Amount of memory in MB")

	flags.StringVar(&bios, "bios", "", "Path to the optional bios binary")

	// Backend configuration
	flags.StringVar(&qemuCmd, "qemu", "", "Path to the qemu binary (otherwise look in $PATH)")
	flags.BoolVar(&qemuDetached, "detached", false, "Set qemu container to run in the background")

	// Networking
	flags.StringVar(&networking, "networking", qemuNetworkingDefault, "Networking mode. Valid options are 'default', 'user', 'bridge[,name]', tap[,name] and 'none'. 'user' uses QEMUs userspace networking. 'bridge' connects to a preexisting bridge. 'tap' uses a prexisting tap device. 'none' disables networking.`")

	flags.Var(&publishFlags, "publish", "Publish a vm's port(s) to the host (default [])")
}

func Qemu(cmd *cobra.Command, args []string) {
	path := args[0]

	if _, err := os.Stat(path); err != nil {
		log.Fatal(err)
	}
	var publishedPorts []PublishedPort
	for _, publish := range publishFlags {
		p, err := NewPublishedPort(publish)
		if err != nil {
			log.Fatal(err)
		}
		publishedPorts = append(publishedPorts, p)
	}
	opts := []qemu.Option{
		qemu.WithDisks(disks...),
		qemu.WithAccel(accel),
		qemu.WithArch(arch),
		qemu.WithCPUs(cpus),
		qemu.WithMemory(mem),
		qemu.WithNetworking(networking),
		qemu.WithStdin(os.Stdin),
		qemu.WithStdout(os.Stdout),
		qemu.WithStderr(os.Stderr),
		qemu.WithBios(bios),
	}
	if enableGUI {
		opts = append(opts, qemu.WithGUI())
	}
	if qemuDetached {
		opts = append(opts, qemu.WithDetached())
	}
	if err := qemu.Run(cmd.Context(), path, opts...); err != nil {
		log.Fatal(err)
	}
}

func haveKVM() bool {
	_, err := os.Stat("/dev/kvm")
	return !os.IsNotExist(err)
}
