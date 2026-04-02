PKG := `go list -f {{.Dir}} ./...`

LINT_VERSION := v2.8.0

fmt:
	@golangci-lint fmt

lint:
	@golangci-lint version
	@golangci-lint config verify
	@golangci-lint run

test:
	@go test -v ./...

mod:
	@go mod tidy

build:
	@CGO_ENABLED=0 go build -o pcurl ./cmd/pcurl

install:
	@CGO_ENABLED=0 go build -o $(shell go env GOPATH)/bin/pcurl ./cmd/pcurl
	@echo "Installed pcurl to $(shell go env GOPATH)/bin/pcurl"
