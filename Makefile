.PHONY: build test run compose-up compose-down migrate fmt vet

build:
	go build ./...

test:
	go test ./...

run:
	go run .

compose-up:
	docker compose up --build

compose-down:
	docker compose down -v

fmt:
	gofmt -w .

vet:
	go vet ./...
