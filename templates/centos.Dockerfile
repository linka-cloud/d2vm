FROM {{ .Image }}

USER root

RUN sed -i 's/mirrorlist/#mirrorlist/g' /etc/yum.repos.d/CentOS-* && \
    sed -i 's|#baseurl=http://mirror.centos.org|baseurl=http://vault.centos.org|g' /etc/yum.repos.d/CentOS-*

RUN yum update -y

RUN yum install -y \
    kernel \
    systemd \
    NetworkManager \
{{- if .Grub }}
    grub2 \
{{- end }}
    e2fsprogs \
    sudo && \
    systemctl enable NetworkManager && \
    systemctl unmask systemd-remount-fs.service && \
    systemctl unmask getty.target

{{- if not .Grub }}
RUN cd /boot && \
        mv $(find . -name 'vmlinuz-*') /boot/vmlinuz && \
        mv $(find . -name 'initramfs-*.img') /boot/initrd.img
{{- end }}

{{ if .Luks }}
RUN yum install -y cryptsetup && \
    dracut --no-hostonly --regenerate-all --force --install="/usr/sbin/cryptsetup"
{{ else }}
RUN dracut --no-hostonly --regenerate-all --force
{{ end }}

{{ if .Password }}RUN echo "root:{{ .Password }}" | chpasswd {{ end }}
