.PHONY: help build test test-integration lint docker-up docker-down migrate-up migrate-down generate seed clean dev dev-down dev-reset ensure-env

## help: list available make targets (default)
help:
	@echo "Quorant – available make targets:"
	@echo ""
	@echo "  build             compile quorant-api and quorant-worker binaries"
	@echo "  test              run unit tests (short mode)"
	@echo "  test-integration  run all tests including integration tests"
	@echo "  lint              run golangci-lint"
	@echo "  docker-up         start docker compose services"
	@echo "  docker-down       stop docker compose services"
	@echo "  migrate-up        apply pending database migrations via the compose atlas service"
	@echo "  migrate-down      roll back database migrations (placeholder)"
	@echo "  generate          run code generators, e.g. sqlc (placeholder)"
	@echo "  seed              load development seed data"
	@echo "  clean             remove compiled binaries"
	@echo "  dev               one-command bootstrap: services + migrate + seed + live reload"
	@echo "  dev-down          stop the compose stack"
	@echo "  dev-reset         drop volumes and re-bootstrap from scratch"
	@echo ""

## build: compile both Go binaries into backend/bin/
build:
	cd backend && go build -o bin/quorant-api ./cmd/quorant-api
	cd backend && go build -o bin/quorant-worker ./cmd/quorant-worker

## test: run unit tests in short mode
test:
	cd backend && go test ./... -short -count=1

## test-integration: run all tests including integration tests
test-integration:
	cd backend && go test ./... -count=1 -tags=integration

## lint: run golangci-lint (prints a helpful message if not installed)
lint:
	@if command -v golangci-lint > /dev/null 2>&1; then \
		cd backend && golangci-lint run ./...; \
	else \
		echo "golangci-lint is not installed."; \
		echo "Install it from https://golangci-lint.run/usage/install/"; \
		echo "  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$$(go env GOPATH)/bin"; \
	fi

## docker-up: start docker compose services in detached mode
docker-up:
	docker compose up -d

## docker-down: stop docker compose services
docker-down:
	docker compose down

## migrate-up: apply pending database migrations via the compose atlas service
migrate-up:
	docker compose run --rm atlas migrate apply --env compose

## migrate-down: roll back database migrations (placeholder – implemented in Task 12)
migrate-down:
	@echo "TODO: atlas migrate down"

## generate: run code generators such as sqlc (placeholder)
generate:
	@echo "TODO: go generate ./..."

## seed: load development seed data
seed: ## Load development seed data
	docker compose exec -T postgres psql -U quorant -d quorant_dev < backend/seeds/seed.sql

## clean: remove compiled binaries
clean:
	rm -rf backend/bin/

## ensure-env: copy .env.example to .env if absent (no-op otherwise)
ensure-env:
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo ".env created from .env.example — review and adjust if needed."; \
	fi

## dev: full bootstrap — services up healthy, migrate, seed, run api+worker with live reload
dev: ensure-env
	docker compose up -d --wait
	$(MAKE) migrate-up
	$(MAKE) seed
	./scripts/dev.sh

## dev-down: stop the compose stack (does nothing if not running)
dev-down:
	docker compose down

## dev-reset: drop volumes and re-bootstrap from scratch
dev-reset:
	docker compose down -v
	$(MAKE) dev
