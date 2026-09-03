BINARY := url-shortener
CMD    := ./cmd

include .env
export

.PHONY: build run test lint fmt vet tidy clean db-up db-down migrate-up migrate-down migrate-create bench-write bench-read bench-write-duration bench-read-duration swagger

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
	docker compose up -d postgres postgres-replica redis

db-down:
	docker compose down

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

migrate-create:
	migrate create -ext sql -dir migrations -seq $(name)

swagger:
	swag init -g cmd/main.go -o internal/docs

n := 10000
c := 50
z := 30s

bench-write:
	mkdir -p bench-runs
	hey -n $(n) -c $(c) -m POST \
		-H "Content-Type: application/json" \
		-d '{"url":"https://example.com"}' \
		http://localhost:$(HTTP_PORT)/api/shorten \
		| tee bench-runs/write-n$(n)-c$(c)-$(shell date +%Y-%m-%d-%H-%M-%S).txt

bench-read:
	mkdir -p bench-runs
	hey -n $(n) -c $(c) -disable-redirects \
		http://localhost:$(HTTP_PORT)/api/$(code) \
		| tee bench-runs/read-n$(n)-c$(c)-$(shell date +%Y-%m-%d-%H-%M-%S).txt

bench-write-duration:
	mkdir -p bench-runs
	hey -z $(z) -c $(c) -m POST \
		-H "Content-Type: application/json" \
		-d '{"url":"https://example.com"}' \
		http://localhost:$(HTTP_PORT)/api/shorten \
		| tee bench-runs/write-z$(z)-c$(c)-$(shell date +%Y-%m-%d-%H-%M-%S).txt

bench-read-duration:
	mkdir -p bench-runs
	hey -z $(z) -c $(c) -disable-redirects \
		http://localhost:$(HTTP_PORT)/api/$(code) \
		| tee bench-runs/read-z$(z)-c$(c)-$(shell date +%Y-%m-%d-%H-%M-%S).txt

profile-cpu:
	go tool pprof "http://localhost:6060/debug/pprof/profile?seconds=30"

profile-allocs:
	go tool pprof http://localhost:6060/debug/pprof/allocs

profile-serve:
	go tool pprof -http=:3000 $(path)
