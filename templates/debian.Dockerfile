FROM {{ .Image }}

USER root

RUN apt-get -y update && \
    DEBIAN_FRONTEND=noninteractive apt-get -y install --no-install-recommends \
      linux-image-amd64

RUN DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      systemd-sysv \
      systemd \
      dbus \
      iproute2 \
      udhcpc \
      iputils-ping

RUN systemctl preset-all

RUN echo "root:{{- if .Password}}{{ .Password}}{{- else}}root{{- end}}" | chpasswd
