FROM rockylinux/rockylinux:10

RUN dnf update -y
RUN dnf install -y qemu-guest-agent openssh-server && \
    echo "PermitRootLogin yes" >> /etc/ssh/sshd_config && \
    systemctl enable dbus.service && \
    systemctl set-default graphical.target
