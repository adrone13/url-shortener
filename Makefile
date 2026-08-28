BINARY := url-shortener
CMD    := ./cmd

include .env
export

.PHONY: build run test lint fmt vet tidy clean db-up db-down migrate-up migrate-down migrate-create

build:
	go build -o bin/$(BINARY) $(CMD)

run:
	go run $(CMD)

test:
	go test ./...

lint: vet
	gofmt -l .

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

migrate-create:
	migrate create -ext sql -dir migrations -seq $(name)
