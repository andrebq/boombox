.PHONY: default build run test watch tidy import find

BOOMBOX_AUTH_ROOTKEY?=blmHX4evD5FygUEa3EWxjzuAPF7lC4sKuWBrhgti/20=

default: test

build: ./dist
	go build -o ./dist/boombox ./cmd/boombox

import: build
	./dist/boombox k7 -f ./dist/index.tape i -dir ./testdata/sample-cassettes/index.tape

run: import
	echo "admin" | BOOMBOX_AUTH_ROOTKEY=${BOOMBOX_AUTH_ROOTKEY} ./dist/boombox programs auth -t ./dist/index.tape register -u "admin"
	BOOMBOX_AUTH_ROOTKEY=${BOOMBOX_AUTH_ROOTKEY} bash boombox-launcher.sh

pattern?=println(
find:
	find . -name '*.go' -type f -exec grep -n '$(pattern)' {} "+" || true

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
