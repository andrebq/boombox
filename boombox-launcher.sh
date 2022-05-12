#!/usr/bin/env bash

set -eou pipefail

mkdir -p ./localfiles

[[ -f ./localfiles/api.pid ]] && {
	kill $(cat ./localfiles/api.pid) || true
}

[[ -f ./localfiles/query.pid ]] && {
	kill $(cat ./localfiles/query.pid) || true
}

./dist/boombox serve api -bind localhost:7008 -tape ./dist/index.tape &
echo $! > ./localfiles/api.pid

./dist/boombox serve query -bind localhost:7009 -tape ./dist/index.tape &
echo $! > ./localfiles/query.pid

./dist/boombox serve router -bind localhost:7007 --api-endpoint http://localhost:7008/ \
  --query-endpoint http://localhost:7009/ || true

kill $(cat ./localfiles/query.pid) || true
kill $(cat ./localfiles/api.pid) || true
