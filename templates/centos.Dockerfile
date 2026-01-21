FROM {{ .Image }} AS rootfs

USER root

{{ $version := atoi .Release.VersionID }}

{{ if le $version 8 }}
RUN sed -i 's/mirrorlist/#mirrorlist/g' /etc/yum.repos.d/CentOS-* && \
    sed -i 's|#baseurl=http://mirror.centos.org|baseurl=http://vault.centos.org|g' /etc/yum.repos.d/CentOS-*
{{ end }}

# See https://bugzilla.redhat.com/show_bug.cgi?id=1917213
RUN yum install -y \
    kernel \
    systemd \
    NetworkManager \
    e2fsprogs \
    sudo && \
    systemctl enable NetworkManager && \
    systemctl unmask systemd-remount-fs.service && \
    systemctl unmask getty.target && \
    find /boot -type l -exec rm {} \;

{{- if .GrubBIOS }}
RUN yum install -y grub2
{{- end }}
{{- if .GrubEFI }}
RUN yum install -y grub2 grub2-efi-x64 grub2-efi-x64-modules
{{- end }}

{{ if .Luks }}
RUN yum install -y cryptsetup && \
    dracut --no-hostonly --regenerate-all --force --install="/usr/sbin/cryptsetup"
{{ else }}
RUN dracut --no-hostonly --regenerate-all --force
{{ end }}

{{ if .Password }}RUN echo "root:{{ .Password }}" | chpasswd {{ end }}

{{- if not .Grub }}
RUN cd /boot && \
        mv $(find {{ if le $version 8 }}.{{ else }}/{{ end }} -name 'vmlinuz*') /boot/vmlinuz && \
        mv $(find . -name 'initramfs-*.img') /boot/initrd.img
{{- end }}

RUN yum clean all && \
    rm -rf /var/cache/yum

FROM scratch

COPY --from=rootfs / /
