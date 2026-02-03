FROM docker.io/library/golang:1.25.6-alpine3.23 AS build

RUN apk add --no-cache \
	build-base=~0.5 \
	cmake=~4.1 \
	python3=~3.12 \
	pkgconf=~2.5

WORKDIR /src
COPY . /src

RUN make -C git2go install-static && \
    go build -tags static .

FROM docker.io/library/alpine:3.23 AS run

COPY --from=build /src/gitr-backup /
LABEL org.opencontainers.image.source https://github.com/alixinne/gitr-backup

ENTRYPOINT ["/gitr-backup"]
