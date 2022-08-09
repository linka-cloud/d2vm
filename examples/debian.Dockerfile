FROM debian

RUN apt update && apt install -y openssh-server systemctl && \
    systemctl enable ssh && \
    echo "PermitRootLogin yes" >> /etc/ssh/sshd_config
