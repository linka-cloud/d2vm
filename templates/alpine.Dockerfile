FROM {{ .Image }}

USER root

RUN apk update --no-cache && \
    apk add \
      util-linux \
      linux-virt \
{{- if ge .Release.VersionID "3.17" }}
      busybox-openrc \
      busybox-mdev-openrc \
      busybox-extras-openrc \
      busybox-mdev-openrc \
{{- else }}
      busybox-initscripts \
{{- end }}
      openrc

RUN for s in bootmisc hostname hwclock modules networking swap sysctl urandom syslog; do rc-update add $s boot; done
RUN for s in devfs dmesg hwdrivers mdev; do rc-update add $s sysinit; done

{{ if .Password }}RUN echo "root:{{ .Password }}" | chpasswd {{ end }}

{{ if eq .NetworkManager "ifupdown"}}
RUN apk add --no-cache ifupdown-ng
RUN mkdir -p /etc/network && printf '\
auto eth0\n\
allow-hotplug eth0\n\
iface eth0 inet dhcp\n\
' > /etc/network/interfaces
{{ end }}
