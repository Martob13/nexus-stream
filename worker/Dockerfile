FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download || true
COPY cmd ./cmd
RUN CGO_ENABLED=0 GOOS=linux go build -o /worker ./cmd/main.go

FROM alpine:latest
WORKDIR /
COPY --from=builder /worker /worker
CMD ["/worker"]
