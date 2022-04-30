.PHONY: default build run test watch tidy

default: test

build: ./dist
	go build -o ./dist/boombox ./cmd/boombox

run: build
	./dist/boombox serve

test:
	go test ./...

watch:
	modd -f modd.conf

tidy:
	go mod tidy
	go fmt ./...


### File Targets
./dist:
	mkdir -p ./dist
