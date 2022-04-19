FROM alpine

RUN apk add --no-cache && \
    echo "PermitRootLogin yes" >> /etc/ssh/sshd_config
