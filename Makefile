.PHONY: run demo test lint build docker-up docker-down

run:
	LOG_FORMAT=text go run ./cmd/bookshelf

# Fresh database with the demo library.
demo:
	rm -f demo.db demo.db-shm demo.db-wal
	DB_PATH=demo.db SEED_DEMO=true LOG_FORMAT=text go run ./cmd/bookshelf

test:
	go test ./...

lint:
	go vet ./...
	golangci-lint run ./...

build:
	CGO_ENABLED=0 go build -trimpath -o bin/bookshelf ./cmd/bookshelf

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down
