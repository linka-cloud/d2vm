FROM {{ .Image }}

USER root

RUN apt-get update -y && \
  apt-get -y install \
  linux-image-virtual


RUN DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      systemd-sysv \
      systemd \
      dbus \
      udhcpc \
      iproute2 \
      iputils-ping

RUN systemctl preset-all

RUN echo "root:{{- if .Password}}{{ .Password}}{{- else}}root{{- end}}" | chpasswd
