FROM {{ .Image }}

USER root

RUN apt-get -y update && \
    DEBIAN_FRONTEND=noninteractive apt-get -y install --no-install-recommends \
{{- if .Luks }}
      cryptsetup \
{{- end }}
      linux-image-amd64

RUN DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      systemd-sysv \
      systemd \
      dbus \
      iproute2 \
      isc-dhcp-client \
      iputils-ping

RUN systemctl preset-all

{{ if .Password }}RUN echo "root:{{ .Password }}" | chpasswd {{ end }}

{{ if eq .NetworkManager "netplan" }}
RUN apt install -y netplan.io
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
RUN if [ -z "$(apt-cache madison ifupdown2 2> /dev/nul)" ]; then apt install -y ifupdown; else apt install -y ifupdown2; fi
RUN mkdir -p /etc/network && printf '\
auto eth0\n\
allow-hotplug eth0\n\
iface eth0 inet dhcp\n\
' > /etc/network/interfaces
{{ end }}
