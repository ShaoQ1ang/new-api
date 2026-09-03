#!/usr/bin/env bash

set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
compose_file="${repo_dir}/deploy/newapi-local/docker-compose.postgres.yml"
cache_root="${BUILDX_CACHE_DIR:-${repo_dir}/data/.buildx-cache/new-api-classic}"

case "$(uname -m)" in
  arm64) native_platform="linux/arm64" ;;
  x86_64) native_platform="linux/amd64" ;;
  *) echo "Unsupported host architecture: $(uname -m)" >&2; exit 1 ;;
esac

target_platform="${TARGET_PLATFORM:-${native_platform}}"
cache_name="${target_platform//\//-}"
cache_current="${cache_root}/${cache_name}"
cache_next="${cache_root}/${cache_name}-next"
deploy_after_build="${DEPLOY_AFTER_BUILD:-}"
if [[ -z "${deploy_after_build}" ]]; then
  [[ "${target_platform}" == "${native_platform}" ]] && deploy_after_build=true || deploy_after_build=false
fi

if [[ "${deploy_after_build}" == "true" && "${target_platform}" != "${native_platform}" ]]; then
  echo "Refusing to deploy ${target_platform} image on ${native_platform} host." >&2
  exit 1
fi

if [[ "${target_platform}" == "${native_platform}" ]]; then
  image_tag="${IMAGE_TAG:-new-api:local}"
  runtime_image="${RUNTIME_IMAGE:-debian:bookworm-slim-local-cache}"
else
  image_tag="${IMAGE_TAG:-new-api:classic-${target_platform##*/}}"
  runtime_image="${RUNTIME_IMAGE:-debian:bookworm-slim}"
fi

mkdir -p "${cache_root}"
rm -rf "${cache_next}"
cache_args=(--cache-to "type=local,dest=${cache_next},mode=max")
for existing_cache in "${cache_root}"/linux-*; do
  [[ -f "${existing_cache}/index.json" ]] || continue
  cache_args+=(--cache-from "type=local,src=${existing_cache}")
done

docker buildx build \
  --platform "${target_platform}" \
  --load \
  --pull=false \
  --build-arg "BUN_IMAGE=${BUN_IMAGE:-oven/bun:local-cache}" \
  --build-arg "GO_IMAGE=${GO_IMAGE:-golang:1.26.1-local-cache}" \
  --build-arg "RUNTIME_IMAGE=${runtime_image}" \
  --build-arg "BUN_REGISTRY=${BUN_REGISTRY:-https://registry.npmmirror.com}" \
  --build-arg "GOPROXY=${GOPROXY:-https://goproxy.cn,direct}" \
  --build-arg "GOSUMDB=${GOSUMDB:-sum.golang.google.cn}" \
  --build-arg "VITE_HOME_ENTRY=${VITE_HOME_ENTRY:-en}" \
  "${cache_args[@]}" \
  -f "${repo_dir}/deploy/newapi-local/Dockerfile.classic" \
  -t "${image_tag}" \
  "${repo_dir}"

rm -rf "${cache_current}"
mv "${cache_next}" "${cache_current}"

if [[ "${deploy_after_build}" != "true" ]]; then
  docker image inspect "${image_tag}" \
    --format 'image={{index .RepoTags 0}} architecture={{.Architecture}} id={{.Id}}'
  exit 0
fi

if [[ "${image_tag}" != "new-api:local" ]]; then
  docker tag "${image_tag}" new-api:local
fi

docker compose --env-file "${repo_dir}/.env" \
  -f "${compose_file}" up -d --no-deps --no-build new-api

for _ in {1..20}; do
  health_state="$(docker inspect new-api-local --format '{{.State.Health.Status}}' 2>/dev/null || true)"
  [[ "${health_state}" == "healthy" ]] && break
  sleep 3
done

curl -fsS http://127.0.0.1:3000/api/status >/dev/null
docker compose --env-file "${repo_dir}/.env" -f "${compose_file}" ps new-api
