.PHONY: test race vet build check
test:
	go test ./...
race:
	go test -race ./...
vet:
	go vet ./...
build:
	go build -o bin/access-monitor ./cmd/access-monitor
check: test race vet build
