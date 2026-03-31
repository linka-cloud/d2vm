FROM rockylinux/rockylinux:10

RUN dnf update -y
RUN dnf install -y qemu-guest-agent openssh-server && \
    echo "PermitRootLogin yes" >> /etc/ssh/sshd_config && \
    systemctl enable dbus.service && \
    systemctl set-default graphical.target

RUN echo "NETWORKING=yes" >> /etc/sysconfig/network && \
    echo -e 'DEVICE="eth0"\nONBOOT="yes"\nBOOTPROTO="dhcp"\n' > /etc/sysconfig/network-scripts/ifcfg-eth0
