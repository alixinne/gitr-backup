FROM docker.io/library/golang:1.20.3-alpine3.17 AS build

RUN apk add --no-cache \
	build-base=~0.5 \
	cmake=~3.24 \
	python3=~3.10 \
	pkgconf=~1.9

WORKDIR /src
COPY . /src

RUN make -C git2go install-static && \
    go build -tags static .

FROM docker.io/library/alpine:3.17 AS run

COPY --from=build /src/gitr-backup /
LABEL org.opencontainers.image.source https://github.com/alixinne/gitr-backup

ENTRYPOINT ["/gitr-backup"]
