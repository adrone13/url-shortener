BINARY := url-shortener
CMD    := ./cmd

.PHONY: build run test lint fmt vet tidy clean

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
