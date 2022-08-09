FROM alpine

RUN apk add --no-cache openrc openssh-server && \
    rc-update add sshd default && \
    echo "PermitRootLogin yes" >> /etc/ssh/sshd_config
