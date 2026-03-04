BINARY_NAME=ask

.PHONY: all build build-desktop install-wails check-wails test clean run deps fmt vet lint install coverage

all: test build

build:
	go build -o $(BINARY_NAME) main.go

build-desktop: check-wails
	@echo "Building desktop application..."
	wails build

install-wails:
	@echo "Installing Wails..."
	go install github.com/wailsapp/wails/v2/cmd/wails@latest

check-wails:
	@which wails > /dev/null || (echo "Wails not found. Please run 'make install-wails'" && exit 1)

test:
	go test -v ./...

clean:
	go clean
	rm -f $(BINARY_NAME)

run:
	go run main.go

deps:
	go mod download

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

install:
	go install

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
