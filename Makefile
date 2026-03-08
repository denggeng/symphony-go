.PHONY: fmt test run docker-build docker-up docker-down

fmt:
	gofmt -w .

test:
	GOPROXY=https://proxy.golang.org,direct go test ./...

run:
	go run ./cmd/symphonyd


docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down
