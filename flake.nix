{
  description = "Basic flake and files - change me";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
    treefmt-nix = {
      url = "github:numtide/treefmt-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      gomod2nix,
      ...
    }@inputs:
    let
      supportedSystems = [ "x86_64-linux" ];

      pkgsFor =
        system:
        let pkgs = import nixpkgs {
          inherit system;
          # for terraform, maybe this will be opentofu someday
          config.allowUnfree = true;
        };
        in 
        pkgs.extend (
          nixpkgs.lib.composeManyExtensions [ gomod2nix.overlays.default ]
        );

      eachSystem = f: nixpkgs.lib.genAttrs supportedSystems (system: f (pkgsFor system));

      treefmtEval = eachSystem (pkgs: inputs.treefmt-nix.lib.evalModule pkgs ./treefmt.nix);

      goPackageFor =
        pkgs:
        pkgs.buildGoApplication {
          pname = "taiga";
          version = "0.1";
          src = ./.;
          modules = ./gomod2nix.toml;
        };
      otherGoPackageFor =
        pkgs:

        pkgs.buildGoModule {
          src = ./.;
          pname = "taiga";
          version = "0.1";

          vendorHash = "sha256-R3zS72aVorekTiJ9iIGI8jMoeVRcXdo/CglvuiRoWnc=";

        };
      dockerImageFor =
        pkgs:
        let
          app = goPackageFor pkgs;
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
          tag = "latest";
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
      checks = eachSystem (pkgs: {

        formatting = treefmtEval.${pkgs.system}.config.build.check self;
        # go-lint = goLintFor pkgs;
        # go-test = goTestFor pkgs;

      });
      # nix fmt formatter
      formatter = eachSystem (pkgs: treefmtEval.${pkgs.system}.config.build.wrapper);

      packages = eachSystem (
        pkgs:

        let
          go-package = goPackageFor pkgs;
          docker-image = dockerImageFor pkgs;
          go-module = otherGoPackageFor pkgs;
        in
        {
          default = go-package;
          inherit go-package;
          inherit go-module;
          inherit docker-image;
        }
      );

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
            #terraform
            pkgs.terraform-ls
            pkgs.terraform

            pkgs.awscli2
          ];

          # Will be executed before entering the shell
          # or running a command
          shellHook = '''';
        };
      });
    };
}
