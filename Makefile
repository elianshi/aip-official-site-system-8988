.PHONY: test check build run migrate seed
test:
	go test -race -coverprofile=coverage.out ./...
check:
	go vet ./...
	git diff --check
build:
	go build -o bin/api ./cmd/api
	go build -o bin/migrate ./cmd/migrate
run:
	go run ./cmd/api
migrate:
	go run ./cmd/migrate
seed:
	go run ./cmd/migrate -seed
