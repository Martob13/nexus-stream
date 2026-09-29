# Stage 1: Build Rust static library
FROM rust:alpine AS rust-builder
RUN apk add --no-cache musl-dev
WORKDIR /build/core-rs
COPY core-rs/Cargo.toml core-rs/Cargo.lock* ./
COPY core-rs/src ./src
COPY core-rs/include ./include
RUN cargo build --release

# Stage 2: Build Go CGO binary with statically linked Rust
FROM golang:1.22-alpine AS go-builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /build

COPY --from=rust-builder /build/core-rs/target/release/libcore_parser.a ./core-rs/target/release/
COPY --from=rust-builder /build/core-rs/include ./core-rs/include

COPY worker/go.mod worker/go.sum* ./worker/
WORKDIR /build/worker
RUN go mod download

COPY worker/ ./
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w -extldflags '-static'" -o /bin/worker ./cmd/main.go

# Stage 3: Minimal non-root runtime container
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata && adduser -D -u 1000 appuser
WORKDIR /app
COPY --from=go-builder /bin/worker /app/worker

USER appuser
CMD ["/app/worker"]
