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
    { nixpkgs, gomod2nix, ... }@inputs:
    let
      supportedSystems = [ "x86_64-linux" ];
      pkgsFor =
        system:
        nixpkgs.legacyPackages.${system}.extend (
          nixpkgs.lib.composeManyExtensions [ gomod2nix.overlays.default ]
          # nixpkgs.lib.composeManyExtensions ([ ] ++ builtins.attrValues self.overlays)
        );
      eachSystem = f: nixpkgs.lib.genAttrs supportedSystems (system: f (pkgsFor system));

      treefmtEval = eachSystem (pkgs: inputs.treefmt-nix.lib.evalModule pkgs ./treefmt.nix);
    in
    {
      # nix fmt formatter
      formatter = eachSystem (pkgs: treefmtEval.${pkgs.system}.config.build.wrapper);

      # default devshell

      devShells = eachSystem (pkgs: {
        default = pkgs.mkShell {
          packages = [
            pkgs.just
            pkgs.gomod2nix
            pkgs.go
            pkgs.gopls
            pkgs.gotools
            pkgs.go-tools
          ];

          # Will be executed before entering the shell
          # or running a command
          shellHook = '''';
        };
      });
    };
}
