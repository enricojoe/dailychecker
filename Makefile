.PHONY: run build test db-up db-down docker-build docker-up docker-down

COMPOSE := docker compose -f docker/docker-compose.yml

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
	$(COMPOSE) up -d db

## db-down: stop and remove Docker containers
db-down:
	$(COMPOSE) down

## docker-env: create docker/.env from the template if it doesn't exist
docker-env:
	@test -f docker/.env || (cp docker/.env.example docker/.env && echo "created docker/.env from template")

## docker-build: build the backend + frontend images (full stack)
docker-build: docker-env
	$(COMPOSE) --profile full build

## docker-up: run the full stack (db + backend + frontend) detached
docker-up: docker-env
	$(COMPOSE) --profile full up -d --build

## docker-down: stop and remove the full stack
docker-down:
	$(COMPOSE) --profile full down
