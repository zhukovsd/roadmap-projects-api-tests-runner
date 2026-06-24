FROM golang:1.26 AS builder

WORKDIR /app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# static binary
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

RUN go build -o test-runner ./cmd

FROM alpine:3

WORKDIR /app

COPY --from=builder app/test-runner .
COPY migrations ./migrations

EXPOSE 8080

ENTRYPOINT ["./test-runner"]
