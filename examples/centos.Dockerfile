FROM centos

RUN sed -i 's/mirrorlist/#mirrorlist/g' /etc/yum.repos.d/CentOS-* && \
    sed -i 's|#baseurl=http://mirror.centos.org|baseurl=http://vault.centos.org|g' /etc/yum.repos.d/CentOS-*

RUN yum update -y
RUN yum install -y qemu-guest-agent openssh-server && \
    echo "PermitRootLogin yes" >> /etc/ssh/sshd_config && \
    systemctl enable dbus.service && \
    systemctl set-default graphical.target

RUN echo "NETWORKING=yes" >> /etc/sysconfig/network && \
    echo -e 'DEVICE="eth0"\nONBOOT="yes"\nBOOTPROTO="dhcp"\n' > /etc/sysconfig/network-scripts/ifcfg-eth0
