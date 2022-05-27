.PHONY: default build run test watch tidy

BOOMBOX_AUTH_ROOTKEY?=blmHX4evD5FygUEa3EWxjzuAPF7lC4sKuWBrhgti/20=

default: test

build: ./dist
	go build -o ./dist/boombox ./cmd/boombox

run: build
	./dist/boombox k7 -f ./dist/index.tape i -dir ./testdata/sample-cassettes/index.tape
	BOOMBOX_AUTH_ROOTKEY=${BOOMBOX_AUTH_ROOTKEY} bash boombox-launcher.sh

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
