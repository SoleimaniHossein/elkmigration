run: clean
	docker compose up redis -d
	go run main.go
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