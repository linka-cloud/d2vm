FROM {{ .Image }}

USER root

RUN apt-get update -y && \
  DEBIAN_FRONTEND=noninteractive apt-get -y install --no-install-recommends \
  linux-image-virtual \
  initramfs-tools \
  systemd-sysv \
  systemd \
  dbus \
  udhcpc \
  iproute2 \
  iputils-ping

RUN systemctl preset-all

RUN echo "root:{{- if .Password}}{{ .Password}}{{- else}}root{{- end}}" | chpasswd
