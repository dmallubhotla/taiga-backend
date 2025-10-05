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
