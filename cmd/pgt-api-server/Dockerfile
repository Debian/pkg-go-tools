# Start with busybox, but with libc.so.6
FROM busybox:ubuntu-14.04

MAINTAINER Michael Stapelberg <stapelberg@debian.org>

# So that we can run as unprivileged user inside the container.
RUN echo 'nobody:x:99:99:nobody:/:/bin/sh' >> /etc/passwd

USER nobody

ADD ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ADD pgt-api-server /usr/bin/pgt-api-server

EXPOSE 8080
EXPOSE 8081

ENTRYPOINT ["/usr/bin/pgt-api-server", "-listen=:8081", "-listen_challenge=:8080"]
