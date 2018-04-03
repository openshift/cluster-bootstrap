FROM fedora:27

ENV DOCKER_VERSION "18.03.0-ce"

## Update and install Development tools
RUN dnf -y update && \
    dnf -y install which rsync git net-tools wget && \
    dnf clean all

## Docker client install
RUN curl -L -o /tmp/docker-$DOCKER_VERSION.tgz https://download.docker.com/linux/static/stable/x86_64/docker-$DOCKER_VERSION.tgz && \
    tar -xz -C /tmp -f /tmp/docker-$DOCKER_VERSION.tgz && \
    mv /tmp/docker/docker /usr/bin && \
    rm -rf /tmp/docker-$DOCKER_VERSION /tmp/docker
