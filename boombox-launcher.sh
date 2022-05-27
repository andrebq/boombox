#!/usr/bin/env bash

set -eou pipefail

mkdir -p ./localfiles

[[ -f ./localfiles/api.pid ]] && {
	kill $(cat ./localfiles/api.pid) 2> /dev/null || true
}

[[ -f ./localfiles/query.pid ]] && {
	kill $(cat ./localfiles/query.pid) 2> /dev/null || true
}

./dist/boombox serve api public -bind localhost:7008 -tape ./dist/index.tape &
echo $! > ./localfiles/api.pid

./dist/boombox serve query -bind localhost:7009 -tape ./dist/index.tape &
echo $! > ./localfiles/query.pid

./dist/boombox serve api private -bind localhost:7010 -tape ./dist/index.tape -auth index -api-prefix /.auth &
echo $! > ./localfiles/private-api.pid

./dist/boombox serve router -bind localhost:7007 --api-endpoint http://localhost:7008/ \
  --query-endpoint http://localhost:7009/ \
  --admin-endpoint http://localhost:7010/.auth || true

kill $(cat ./localfiles/query.pid) || true
kill $(cat ./localfiles/api.pid) || true
kill $(cat ./localfiles/private-api.pid) || true
