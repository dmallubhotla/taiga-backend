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

#

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
