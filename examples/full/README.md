# ZSH Workstation example

This example demonstrate the setup of a ZSH workstation with *cloud-init* support.

*Dockerfile*
```dockerfile
FROM ubuntu

# Install some system packages
RUN apt update && DEBIAN_FRONTEND=noninteractive apt install -y --no-install-recommends \
      qemu-guest-agent \
      ca-certificates \
      dnsutils \
      sudo \
      openssh-server

# Add a utility script to resize serial terminal
COPY resize /usr/local/bin/

# User setup variables
ARG USER=d2vm
ARG PASSWORD=d2vm
ARG SSH_KEY=https://github.com/${USER}.keys

# Setup user environment
RUN DEBIAN_FRONTEND=noninteractive apt install -y --no-install-recommends \
      bash-completion \
      curl \
      zsh \
      git \
      vim \
      tmux \
      htop \
      lsb-core \
      cloud-init \
      cloud-guest-utils

# Create user with sudo privileged and passwordless sudo
RUN useradd ${USER} -m -s /bin/zsh -G sudo && \
      echo "${USER}:${PASSWORD}" | chpasswd && \
      sed -i 's|ALL=(ALL:ALL) ALL|ALL=(ALL:ALL) NOPASSWD: ALL|g' /etc/sudoers

# Add ssh public keys
ADD ${SSH_KEY} /home/${USER}/.ssh/authorized_keys
# Setup permission on .ssh directory
RUN chown -R ${USER}:${USER} /home/${USER}/.ssh

# Run everything else as the created user
USER ${USER}

# Setup zsh environment
RUN bash -c "$(curl -fsSL https://gist.githubusercontent.com/Adphi/f3ce3cc4b2551c437eb667f3a5873a16/raw/be05553da87f6e9d8b0d290af5aa036d07de2e25/env.setup)"
# Setup tmux environment
RUN bash -c "$(curl -fsSL https://gist.githubusercontent.com/Adphi/765e9382dd5e547633be567e2eb72476/raw/a3fe4b3f35e598dca90e2dd45d30dc1753447a48/tmux-setup)"
# Setup auto login serial console
RUN sudo sed -i "s|ExecStart=.*|ExecStart=-/sbin/agetty --autologin ${USER} --keep-baud 115200,38400,9600 \%I \$TERM|" /usr/lib/systemd/system/serial-getty@.service

```

There is no need to configure the network as **d2vm** will generate a *netplan* configuration that use DHCP.

**Build**

```bash
USER=mygithubuser
PASSWORD=mysecurepasswordthatIwillneverusebecauseIuseMostlySSHkeys
OUTPUT=workstation.qcow2

d2vm build -o $OUTPUT --build-arg USER=$USER --build-arg PASSWORD=$PASSWORD --build-arg SSH_KEY=https://github.com/$USER.keys --force -v .
```

Run it:

```bash
d2vm run qemu --mem 4096 --cpus 4 $IMAGE
```
... you should be automatically logged in with a **oh-my-zsh** shell

You should be able to find the ip address inside the VM using:
```bash
hostname -I
# or
ip a show eth0 | grep inet | awk '{print $2}' | cut -d/ -f1
```

And connect using ssh...

In order to quit the terminal you need to shut down the VM with the `poweroff` command:

```bash
sudo poweroff
```

*I hope you will find it useful and that you will have fun...*
