FROM golang as builder

WORKDIR /d2vm

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY . .

RUN go build -o d2vm ./cmd/d2vm

FROM ubuntu

RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get -y install --no-install-recommends \
        util-linux \
        kpartx \
        e2fsprogs \
        xfsprogs \
        mount \
        tar \
        extlinux \
        uuid-runtime \
        qemu-utils

COPY --from=docker:dind /usr/local/bin/docker /usr/local/bin/

COPY --from=builder /d2vm/d2vm /usr/local/bin/

ENTRYPOINT ["/usr/local/bin/d2vm"]
