.PHONY: run test generate docker
run:
	go run ./cmd/server
test:
	go test -race -cover ./...
generate:
	go tool oapi-codegen -config oapi-codegen.yaml spec.yaml
docker:
	docker compose up --build
