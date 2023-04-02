FROM docker.io/library/alpine:3.17 AS run
COPY gitr-backup /
ENTRYPOINT ["/gitr-backup"]
