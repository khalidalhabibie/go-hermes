APP_NAME=go-hermes

.PHONY: run test test-unit test-integration test-postgres-integration tidy migrate-up migrate-down

run:
	go run ./cmd/api

test:
	go test ./...

test-unit:
	go test ./internal/... ./cmd/...

test-integration:
	go test ./tests/...

test-postgres-integration:
	go test ./tests/... -run Postgres

tidy:
	go mod tidy

migrate-up:
	migrate -path migrations -database "$$DB_DSN" up

migrate-down:
	migrate -path migrations -database "$$DB_DSN" down 1
