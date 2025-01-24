run:
	sudo rm -rf ./logs/app.log
	docker compose down redis -v
	docker compose up redis -d
	go run ./cmd/main.go
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