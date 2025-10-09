# by default list all just commands

default:
    just --list

# build
build:
    #!/usr/bin/env bash
    nix build

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
