FROM golang:1.22-bookworm AS builder
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/sstv ./cmd/sstv

FROM debian:bookworm-slim AS runtime
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates ffmpeg \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /out/sstv /usr/local/bin/sstv

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/sstv"]
CMD ["-addr", ":8080"]
