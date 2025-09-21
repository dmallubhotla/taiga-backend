# by default list all just commands
default:
    just --list

# run all tests
test:
    #!/usr/bin/env bash
    set -euxo pipefail

    # would love test: fmt to make sure formatting happens but in WSL formatting is slow...
    # poor filesystem access performance

    echo "testing..."
    nix flake check
    # uv run ruff check src tests

#

# format code
fmt:
    #!/usr/bin/env bash
    set -euxo pipefail
    nix fmt
