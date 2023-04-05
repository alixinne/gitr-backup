FROM docker.io/library/alpine:3.17 AS run
COPY gitr-backup /
LABEL org.opencontainers.image.source https://github.com/vtavernier/gitr-backup
ENTRYPOINT ["/gitr-backup"]
