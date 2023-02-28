FROM {{ .Image }}

USER root

RUN sed -i 's/mirrorlist/#mirrorlist/g' /etc/yum.repos.d/CentOS-* && \
    sed -i 's|#baseurl=http://mirror.centos.org|baseurl=http://vault.centos.org|g' /etc/yum.repos.d/CentOS-*

RUN yum update -y

RUN yum install -y \
    kernel \
    systemd \
    NetworkManager \
    e2fsprogs \
    {{- if .Luks }}
    cryptsetup \
    {{- end }}
    sudo && \
    systemctl enable NetworkManager && \
    systemctl unmask systemd-remount-fs.service && \
    systemctl unmask getty.target

RUN dracut --no-hostonly --regenerate-all --force {{ if .Luks }}--install="/usr/sbin/cryptsetup"{{ end }}&& \
    cd /boot && \
    ln -s $(find . -name 'vmlinuz-*') vmlinuz && \
    ln -s $(find . -name 'initramfs-*.img') initrd.img

{{ if .Password }}RUN echo "root:{{ .Password }}" | chpasswd {{ end }}
