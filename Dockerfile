FROM golang:1.26 AS builder

WORKDIR /app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# static binary
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

RUN go build -o bin/test-runner ./cmd
RUN go test -o bin/currencyexchange.test -c ./tests/currency_exchange/currency_exchange_test.go

FROM golang:1.26

WORKDIR /app

COPY --from=builder app/bin/test-runner .
COPY --from=builder app/bin/currencyexchange.test .
COPY migrations ./migrations

EXPOSE 8080

ENTRYPOINT ["./test-runner"]
