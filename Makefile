.PHONY: fmt test run

fmt:
	gofmt -w .

test:
	GOPROXY=https://proxy.golang.org,direct go test ./...

run:
	go run ./cmd/symphonyd
