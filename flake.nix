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
        nixpkgs.legacyPackages.${system}.extend (
          nixpkgs.lib.composeManyExtensions [ gomod2nix.overlays.default ]
        );
      eachSystem = f: nixpkgs.lib.genAttrs supportedSystems (system: f (pkgsFor system));

      treefmtEval = eachSystem (pkgs: inputs.treefmt-nix.lib.evalModule pkgs ./treefmt.nix);

      goPackageFor =
        pkgs:
        pkgs.buildGoApplication {
          pname = "gomod2nix-example";
          version = "0.1";
          src = ./.;
          modules = ./gomod2nix.toml;
        };

      goTestFor =
        pkgs:
        pkgs.stdenvNoCC.mkDerivation {
          name = "go-test";
          dontBuild = true;
          src = ./.;
          doCheck = true;
          nativeBuildInputs = with pkgs; [
            go
            writableTmpDirAsHomeHook
          ];
          checkPhase = ''
            go test ./...
          '';
          installPhase = ''
            mkdir "$out"
          '';
        };

      goLintFor =
        pkgs:
        pkgs.stdenvNoCC.mkDerivation {
          name = "go-lint";
          dontBuild = true;
          src = ./.;
          doCheck = true;
          nativeBuildInputs = with pkgs; [
            go
            golangci-lint
            writableTmpDirAsHomeHook
          ];
          checkPhase = ''
            golangci-lint run
          '';
          installPhase = ''
            mkdir "$out"
          '';
        };
    in
    {
      checks = eachSystem (pkgs: {

        formatting = treefmtEval.${pkgs.system}.config.build.check self;
        go-lint = goLintFor pkgs;
        go-test = goTestFor pkgs;

      });
      # nix fmt formatter
      formatter = eachSystem (pkgs: treefmtEval.${pkgs.system}.config.build.wrapper);

      packages = eachSystem (pkgs: {
        default = goPackageFor pkgs;
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
          ];

          # Will be executed before entering the shell
          # or running a command
          shellHook = '''';
        };
      });
    };
}
