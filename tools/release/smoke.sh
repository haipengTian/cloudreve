#!/usr/bin/env bash
set -euo pipefail
image=${1:?usage: bash tools/release/smoke.sh image}
name="cloudreve-smoke-$RANDOM-$$"
scratch=$(mktemp -d /tmp/cloudreve-image-smoke.XXXXXX)
cleanup() {
  result=$?
  if ((result != 0)); then docker logs --tail 80 "$name" >&2 || true; fi
  docker rm -f "$name" >/dev/null 2>&1 || true
  # The container creates root-owned private directories in this CI-only mount.
  if [[ $(realpath -m -- "$scratch") != /tmp/cloudreve-image-smoke.* ]]; then
    echo 'Refusing to clean an unexpected smoke directory' >&2
    result=1
  elif ! sudo rm -rf -- "$scratch"; then
    result=1
  fi
  return "$result"
}
trap cleanup EXIT
mkdir -p "$scratch/data"
start() {
  docker run -d --init --name "$name" -p 127.0.0.1::5212 \
    -v "$scratch/data:/cloudreve/data" -e CR_ENABLE_ARIA2=0 "$image" >/dev/null
}
start
ready() {
  local state
  for _ in {1..90}; do
    state=$(docker inspect --format '{{.State.Health.Status}}' "$name")
    if [[ $state == healthy ]]; then return 0; fi
    sleep 2
  done
  return 1
}
ready
refresh_base() {
  port=$(docker port "$name" 5212/tcp | head -1 | awk -F: '{print $NF}')
  base="http://127.0.0.1:$port"
}
refresh_base
curl --fail --silent --show-error "$base/api/v4/site/ping" | jq -e '.code == 0' >/dev/null
curl --fail --silent --show-error "$base/" | grep -q 'assets/index-'
node tools/release/smoke.mjs "$base" "$scratch/account.json" create
test -s "$scratch/data/conf.ini"
test -s "$scratch/data/cloudreve.db"
docker stop --time 30 "$name" >/dev/null
docker rm "$name" >/dev/null
start
ready
refresh_base
curl --fail --silent --show-error "$base/api/v4/site/ping" | jq -e '.code == 0' >/dev/null
node tools/release/smoke.mjs "$base" "$scratch/account.json" verify
echo 'Container startup, embedded frontend and business data after recreation passed.'
