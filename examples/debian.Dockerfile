FROM debian

RUN apt update && apt install -y openssh-server && \
    echo "PermitRootLogin yes" >> /etc/ssh/sshd_config
