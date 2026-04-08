.PHONY: fmt vet test cover build run clean

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

build:
	go build -o subconverter ./cmd/subconverter

run: build
	./subconverter -config configs/base_config.yaml

clean:
	rm -f subconverter coverage.out coverage.html
