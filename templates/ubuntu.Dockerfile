FROM {{ .Image }}

USER root

{{ if le .Release.VersionID "14.04" }}
# restore initctl
RUN rm /sbin/initctl && dpkg-divert --rename --remove /sbin/initctl
# setup ttyS0
RUN cp /etc/init/tty1.conf /etc/init/ttyS0.conf && sed -i s/tty1/ttyS0/g /etc/init/ttyS0.conf
{{ end }}

RUN ARCH="$([ "$(uname -m)" = "x86_64" ] && echo amd64 || echo arm64)"; \
  apt-get update && \
  DEBIAN_FRONTEND=noninteractive apt-get -y install --no-install-recommends \
  linux-image-virtual \
  initramfs-tools \
{{- if .Grub }}
  grub-common \
  grub2-common \
{{- end }}
{{- if .GrubBIOS }}
  grub-pc-bin \
{{- end }}
{{- if .GrubEFI }}
  grub-efi-${ARCH}-bin \
{{- end }}
  dbus \
  isc-dhcp-client \
  iputils-ping && \
  find /boot -type l -exec rm {} \;

{{ if ge .Release.VersionID "14.04" }}
RUN ARCH="$([ "$(uname -m)" = "x86_64" ] && echo amd64 || echo arm64)"; \
  apt-get update && \
  DEBIAN_FRONTEND=noninteractive apt-get -y install --no-install-recommends \
  iproute2
{{ end }}

{{ if ge .Release.VersionID "16.04" }}
RUN ARCH="$([ "$(uname -m)" = "x86_64" ] && echo amd64 || echo arm64)"; \
  apt-get update && \
  DEBIAN_FRONTEND=noninteractive apt-get -y install --no-install-recommends \
  systemd-sysv \
  systemd
{{ end }}

{{ if gt .Release.VersionID "16.04" }}
RUN systemctl preset-all
{{ end }}

{{ if .Password }}RUN echo "root:{{ .Password }}" | chpasswd {{ end }}

{{ if eq .NetworkManager "netplan" }}
RUN apt-get install -y netplan.io
RUN mkdir -p /etc/netplan && printf '\
network:\n\
  version: 2\n\
  renderer: networkd\n\
  ethernets:\n\
    eth0:\n\
      dhcp4: true\n\
      dhcp-identifier: mac\n\
      nameservers:\n\
        addresses:\n\
        - 8.8.8.8\n\
        - 8.8.4.4\n\
' > /etc/netplan/00-netcfg.yaml
{{ else if eq .NetworkManager "ifupdown"}}
RUN if [ -z "$(apt-cache madison ifupdown-ng 2> /dev/nul)" ]; then apt-get install -y ifupdown; else apt-get install -y ifupdown-ng; fi
RUN mkdir -p /etc/network && printf '\
auto eth0\n\
allow-hotplug eth0\n\
iface eth0 inet dhcp\n\
' > /etc/network/interfaces
{{ end }}

{{- if .Luks }}
RUN DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends cryptsetup-initramfs && \
    update-initramfs -u -v
{{- end }}

# needs to be after update-initramfs
{{- if not .Grub }}
RUN mv $(find /boot -name 'vmlinuz-*') /boot/vmlinuz && \
      mv $(find /boot -name 'initrd.img-*') /boot/initrd.img
{{- end }}

RUN apt-get clean && \
    rm -rf /var/lib/apt/lists/*
