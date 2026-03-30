FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN go build -o /bin/go-hermes ./cmd/api

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/go-hermes /app/go-hermes
COPY ./docs /app/docs

EXPOSE 8080

CMD ["/app/go-hermes"]
