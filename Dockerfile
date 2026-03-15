FROM golang:1.26.1-alpine AS builder

WORKDIR /src

COPY . .

WORKDIR /src/kernel

RUN apk add --no-cache build-base
RUN mkdir -p /out && \
    go build -o /out/apid ./cmd/apid && \
    go build -o /out/registryd ./cmd/registryd && \
    go build -o /out/materializerd ./cmd/materializerd && \
    go build -o /out/webd ./cmd/webd

FROM alpine:3.21

RUN addgroup -S app && adduser -S -G app app
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /out/apid /usr/local/bin/apid
COPY --from=builder /out/registryd /usr/local/bin/registryd
COPY --from=builder /out/materializerd /usr/local/bin/materializerd
COPY --from=builder /out/webd /usr/local/bin/webd
COPY . /app

RUN chown -R app:app /app

USER app

ENV AS_REPO_ROOT=/app

CMD ["/usr/local/bin/webd"]
