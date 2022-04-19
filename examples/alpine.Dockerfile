FROM alpine

RUN apk add --no-cache openssh-server && \
    echo "PermitRootLogin yes" >> /etc/ssh/sshd_config
