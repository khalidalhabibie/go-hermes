APP_NAME=go-hermes

.PHONY: run test test-race test-unit test-integration test-postgres-integration lint tidy migrate-up migrate-down

run:
	go run ./cmd/api

test:
	go test ./...

test-race:
	go test -race ./...

test-unit:
	go test ./internal/... ./cmd/...

test-integration:
	go test ./tests/...

test-postgres-integration:
	go test ./tests/... -run Postgres

lint:
	golangci-lint run

tidy:
	go mod tidy

migrate-up:
	migrate -path migrations -database "$$DB_DSN" up

migrate-down:
	migrate -path migrations -database "$$DB_DSN" down 1
