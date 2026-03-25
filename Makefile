.PHONY: up down logs test test-cover generate build lint send-event

## up: start the full stack locally with Docker Compose
up:
	docker compose up --build

## down: stop and remove local Docker Compose stack
down:
	docker compose down

## logs: tail logs from the app service
logs:
	docker compose logs -f app

## test: run all unit tests
test:
	go test ./...

## test-cover: run tests with coverage report
test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

## generate: regenerate all moq mocks
generate:
	go generate ./...

## build: build the binary locally
build:
	CGO_ENABLED=0 go build -o skip-the-line ./cmd/server

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## send-event: send a sample webhook to the running local app.
##   EVENT=pull_request|pull_request_review|pull_request_review_comment
##   Example: make send-event EVENT=pull_request_review
send-event:
	@EVENT=$(EVENT) ./scripts/send-webhook.sh $(EVENT)
