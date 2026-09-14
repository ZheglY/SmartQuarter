SERVICES := max-gateway identity-service issue-service ai-worker community-service
SERVICE ?= max-gateway

.PHONY: test vet build fmt check run up down

test:
	@set -e; for service in $(SERVICES); do (cd services/$$service && GOWORK=off go test -race ./...); done

vet:
	@set -e; for service in $(SERVICES); do (cd services/$$service && GOWORK=off go vet ./...); done

build:
	@set -e; for service in $(SERVICES); do (cd services/$$service && GOWORK=off go build -mod=readonly ./...); done

fmt:
	@gofmt -w services

check: vet test build

run:
	@cd services/$(SERVICE) && go run ./cmd/app

up:
	docker compose -f deploy/docker-compose.yml up --build -d

down:
	docker compose -f deploy/docker-compose.yml down

