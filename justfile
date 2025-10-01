# by default list all just commands

default:
    just --list

# run all tests
test:
    #!/usr/bin/env bash
    set -euxo pipefail

    echo "testing..."
    nix flake check
    go test ./...

# Vet sqlc
vet_sqlc:
    #!/usr/bin/env bash
    set -euxo pipefail

    sqlc diff
    sqlc vet

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

# Generate coverage to ./profile.cov
cover:
    #!/usr/bin/env bash
    set -euxo pipefail

    mkdir -p ./coverage
    go test -coverprofile ./coverage/profile.cov ./...
    go tool cover -html ./coverage/profile.cov -o ./coverage/cover.html
