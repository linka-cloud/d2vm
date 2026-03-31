FROM {{ .Image }} AS rootfs

USER root

# Install base packages
RUN dnf install -y \
    kernel \
    systemd \
    NetworkManager \
    e2fsprogs \
    sudo && \
    systemctl enable NetworkManager && \
    systemctl unmask systemd-remount-fs.service && \
    systemctl unmask getty.target && \
    mkdir -p /boot && \
    find /boot -type l -exec rm {} \;

{{- if .GrubBIOS }}
RUN dnf install -y grub2
{{- end }}
{{- if .GrubEFI }}
RUN dnf install -y grub2 grub2-efi-x64 grub2-efi-x64-modules
{{- end }}

{{ if .Luks }}
RUN dnf install -y cryptsetup && \
    dracut --no-hostonly --regenerate-all --force --install="/usr/sbin/cryptsetup"
{{ else }}
RUN dracut --no-hostonly --regenerate-all --force
{{ end }}

{{ if .Password }}RUN echo "root:{{ .Password }}" | chpasswd {{ end }}

{{- if not .Grub }}
RUN cd /boot && \
        mv $(find / -name 'vmlinuz*') /boot/vmlinuz && \
        mv $(find . -name 'initramfs-*.img' -o -name initrd) /boot/initrd.img
{{- end }}

RUN dnf clean all && \
    rm -rf /var/cache/dnf

FROM scratch

COPY --from=rootfs / /
