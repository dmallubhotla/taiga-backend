# by default list all just commands

default:
    just --list

# build
build:
    #!/usr/bin/env bash
    nix build

build-docker:
    #!/usr/bin/env bash
    nix build .#docker-image

load-docker:
    #!/usr/bin/env bash
    docker load < result

build-load: build-docker load-docker
    echo "Loaded image"

# run all tests
test:
    #!/usr/bin/env bash
    set -euxo pipefail

    echo "testing..."
    nix flake check
    go test -coverprofile ./coverage/profile.cov ./...
    go tool cover -html ./coverage/profile.cov -o ./coverage/cover.html

# Vet sqlc
vet_sqlc:
    #!/usr/bin/env bash
    set -euxo pipefail

    sqlc diff
    sqlc vet

full_test: vet_sqlc test

# format code
fmt:
    #!/usr/bin/env bash
    set -euxo pipefail
    nix fmt

# stupid chores
chores:
    #!/usr/bin/env bash
    set -euxo pipefail
    gomod2nix
    sqlc generate

# update flake inputs
update:
    #!/usr/bin/env bash
    nix flake update

# release: stamp flake.nix, commit, tag, push — all via `hanko seal`.
# Reads .hanko.yaml for stamp-targets + seal config.
# Preview with `just release-plan` before running for real.
release:
    nix develop --command hanko seal

# release-plan: print what `just release` would do without mutating anything.
release-plan:
    nix develop --command hanko seal --dry-run

# Serve using go run
serve:
    #!/usr/bin/env bash
    set -euxo pipefail

    go run cmd/server/main.go

# Generates the rsa keypair for JWT signing.

# ONLY FOR DEVELOPMENT USE
generate_keypair:
    #!/usr/bin/env bash
    set -euxo pipefail

    mkdir -p cert
    ssh-keygen -t rsa -b 4096 -m PEM -f cert/test.key
    openssl rsa -in cert/test.key -pubout -outform PEM -out cert/test.pem

# run a loaded docker image
exec-docker:
    #!/usr/bin/env bash
    set -euxo pipefail

    mkdir -p ./local/docker/filerepo
    docker run -it  -v ./local/docker/config.yaml:/workspace/config.yaml -v ./local/docker/cert:/cert -v ./local/docker/data.db:/workspace/data.db ./local/docker/filerepo:/filerepo taiga /bin/bash
