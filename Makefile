run:
	go run ./cmd/main.go
generate:
	go run ./cmd/data/generator.go
init:
	docker compose build --no-cache
build:
	docker compose build
up:
	docker compose up
down:
	docker compose down
clean:
	docker compose down -v