FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o fileupload ./cmd

FROM alpine:latest

WORKDIR /app

COPY --from=builder /build/fileupload .

CMD ["./fileupload"]
