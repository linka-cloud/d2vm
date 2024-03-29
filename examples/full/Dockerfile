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
