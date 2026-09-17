SERVICES := $(patsubst services/%/go.mod,%,$(wildcard services/*/go.mod))
SERVICE ?= max-gateway

.PHONY: test vet lint build fmt check run up down

test:
	@set -e; for service in $(SERVICES); do (cd services/$$service && GOWORK=off go test ./...); done

vet:
	@set -e; for service in $(SERVICES); do (cd services/$$service && GOWORK=off go vet ./...); done

lint: vet
	@files="$$(gofmt -l services)"; if [ -n "$$files" ]; then printf 'Run gofmt on:\n%s\n' "$$files"; exit 1; fi

build:
	@set -e; for service in $(SERVICES); do (cd services/$$service && GOWORK=off go build -mod=readonly ./...); done

fmt:
	@gofmt -w services

check: lint test

run:
	@cd services/$(SERVICE) && go run ./cmd/app

up:
	docker compose -f deploy/docker-compose.yml up --build -d

down:
	docker compose -f deploy/docker-compose.yml down
