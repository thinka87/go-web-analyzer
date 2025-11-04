.PHONY: run build docker test

run:
	go run .

build:
	go build -o web-analyzer

docker:
	docker build -t web-analyzer:local .

test:
	go test ./...
