FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/bookshelf ./cmd/bookshelf

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
  && adduser -D -H -u 10001 app \
  && mkdir /data && chown app /data
COPY --from=build /out/bookshelf /usr/local/bin/bookshelf
USER app
ENV ADDR=:4409 DB_PATH=/data/bookshelf.db
VOLUME /data
EXPOSE 4409
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=5 \
  CMD wget -qO- http://127.0.0.1:4409/healthz > /dev/null || exit 1
ENTRYPOINT ["/usr/local/bin/bookshelf"]
