.PHONY: run build test test-integration docker-up docker-down

run:
	go run ./cmd/bot

build:
	go build -o bin/bot ./cmd/bot

test:
	go test ./...

test-integration:
	set -a && . ./.env && set +a && go test -tags integration -count=1 -v ./internal/adapter/...

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down
