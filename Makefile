.PHONY: default build run test watch tidy

default: test

build: ./dist
	go build -o ./dist/boombox ./cmd/boombox

run: build
	./dist/boombox k7 -f ./dist/index.tape i -dir ./testdata/sample-cassettes/index.tape
	./dist/boombox serve -tape ./dist/index.tape

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
