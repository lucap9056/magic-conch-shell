FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY v2/go.mod v2/go.sum ./
RUN go mod download

COPY v2/ ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o core ./cmd

ARG GRPC_PROBE_VERSION=v0.4.39
RUN wget -qO /grpc_health_probe \
    https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_PROBE_VERSION}/grpc_health_probe-linux-amd64 \
    && chmod +x /grpc_health_probe

FROM alpine:latest

WORKDIR /app

COPY --from=builder /build/core .
COPY --from=builder /grpc_health_probe .

CMD ["./core"]
