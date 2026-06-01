{
  description = "taiga-backend: Go web API with PostgreSQL/SQLite, JWT auth, and FIT file processing";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    treefmt-nix = {
      url = "github:numtide/treefmt-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    hanko = {
      url = "github:dmallubhotla/hanko";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      gomod2nix,
      hanko,
      ...
    }@inputs:
    let
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
        "x86_64-darwin"
      ];

      pkgsFor =
        system:
        let
          pkgs = import nixpkgs {
            inherit system;
            # for terraform, maybe this will be opentofu someday
            config.allowUnfree = true;
          };
        in
        pkgs.extend (
          nixpkgs.lib.composeManyExtensions [
            gomod2nix.overlays.default
            taigaOverlay
          ]
        );

      eachSystem = f: nixpkgs.lib.genAttrs supportedSystems (system: f (pkgsFor system));

      treefmtEval = eachSystem (pkgs: inputs.treefmt-nix.lib.evalModule pkgs ./treefmt.nix);

      # Stamped by hanko; do not hand-edit (use `just release`).
      version = "0.1.0";
      commonLdflags = hanko.lib.mkGoLdflags { inherit self version; };

      taigaOverlay = final: _prev: {
        taiga = final.buildGoApplication {
          pname = "taiga";
          inherit version;
          src = ./.;
          modules = ./gomod2nix.toml;
          ldflags = commonLdflags;
        };
      };

      dockerImageFor =
        pkgs:
        let
          app = pkgs.taiga;
          migrations-postgres = pkgs.lib.fileset.toSource {
            root = ./.;
            fileset = ./migrations;
          };
          migrations-sqlite = pkgs.lib.fileset.toSource {
            root = ./.;
            fileset = ./migrations-sqlite;
          };
        in
        pkgs.dockerTools.buildLayeredImage {
          name = "taiga";
          # Stamped from the flake's `version` (hanko-managed). The push tags
          # in CI (`:latest`, `:master`, `:sha-…`) are layered on top by skopeo;
          # this is the immutable on-image identity.
          tag = version;
          contents = [
            pkgs.dockerTools.usrBinEnv

            migrations-postgres
            migrations-sqlite

            pkgs.bash
            pkgs.coreutils

            app
          ];
          config = {
            Cmd = [ "/bin/bash" ];
            Env = [
              "PATH=/bin"
            ];
            WorkingDir = "/workspace";
          };
        };

    in
    {

      overlays.default = nixpkgs.lib.composeManyExtensions [
        gomod2nix.overlays.default
        taigaOverlay
      ];

      checks = eachSystem (pkgs: {
        formatting = treefmtEval.${pkgs.stdenv.hostPlatform.system}.config.build.check self;
        # Lint as a check: override the taiga derivation to swap go test for
        # golangci-lint in checkPhase. Reuses the vendored module setup from
        # goConfigHook so no network is needed inside the sandbox.
        golangci-lint = pkgs.taiga.overrideAttrs (old: {
          pname = "taiga-golangci-lint";
          nativeCheckInputs = (old.nativeCheckInputs or [ ]) ++ [ pkgs.golangci-lint ];
          doCheck = true;
          checkPhase = ''
            runHook preCheck
            export GOLANGCI_LINT_CACHE=$TMPDIR/golangci-lint-cache
            golangci-lint run --timeout 5m ./...
            runHook postCheck
          '';
        });
        # Run `go test` against the vendored module via the standard checkPhase.
        go-test = pkgs.taiga.overrideAttrs (_old: {
          pname = "taiga-go-test";
          doCheck = true;
        });
      });

      # nix fmt formatter
      formatter = eachSystem (pkgs: treefmtEval.${pkgs.stdenv.hostPlatform.system}.config.build.wrapper);

      packages = eachSystem (pkgs: {
        default = pkgs.taiga;
        taiga = pkgs.taiga;
        docker-image = dockerImageFor pkgs;
      });

      # default devshell
      devShells = eachSystem (pkgs: {
        default = pkgs.mkShell {
          packages = [
            pkgs.just
            pkgs.gomod2nix
            pkgs.go
            pkgs.gopls
            pkgs.gotools
            pkgs.golangci-lint
            pkgs.go-tools
            pkgs.sqlc
            pkgs.sqlite
            pkgs.postgresql
            pkgs.openssl
            # terraform, maybe opentofu someday
            pkgs.terraform-ls
            # pkgs.terraform

            pkgs.awscli2

            # Release tooling — `just release` invokes `hanko seal`.
            hanko.packages.${pkgs.stdenv.hostPlatform.system}.default
          ];
        };
      });
    };
}
