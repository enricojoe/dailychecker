.PHONY: run build test db-up db-down

## run: start the backend dev server (requires Postgres via db-up)
run:
	cd backend && go run ./cmd/server

## build: compile the backend binary to backend/bin/server
build:
	cd backend && go build -o bin/server ./cmd/server

## test: run all backend tests
test:
	cd backend && go test ./...

## db-up: start Postgres in Docker (detached)
db-up:
	docker compose up -d

## db-down: stop and remove Docker containers
db-down:
	docker compose down
