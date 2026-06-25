FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY v2/go.mod v2/go.sum ./
RUN go mod download

COPY v2/ ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o core ./cmd

FROM alpine:latest

WORKDIR /app

COPY --from=builder /build/core .

CMD ["./core"]
