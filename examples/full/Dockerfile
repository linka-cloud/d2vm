FROM ubuntu

# Install netplan sudo ssh-server and dns utils
RUN apt update && DEBIAN_FRONTEND=noninteractive apt install -y \
      qemu-guest-agent \
      netplan.io \
      dnsutils \
      sudo \
      openssh-server

# Setup default network config
COPY 00-netconf.yaml /etc/netplan/
# Add a utility script to resize serial terminal
COPY resize /usr/local/bin/

# User setup variables
ARG USER=d2vm
ARG PASSWORD=d2vm
ARG SSH_KEY=https://github.com/${USER}.keys

# Setup user environment
RUN DEBIAN_FRONTEND=noninteractive apt install -y \
      bash-completion \
      curl \
      zsh \
      git \
      vim \
      tmux \
      htop

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

