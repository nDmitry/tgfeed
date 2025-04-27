.SILENT:
.PHONY: lint up

lint:
	go tool -modfile=go.tool.mod golangci-lint run ./...

up:
	docker compose up -d
	docker image prune --force
