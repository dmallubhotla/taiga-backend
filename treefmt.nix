# treefmt.nix
{ ... }:
{
  projectRootFile = "treefmt.nix";
  settings.global.excludes = [
    "*.toml"
    "*.txt"
    ".gitattributes"
    "CLAUDE.md"
    ".python-version"
  ];

  programs.deadnix.enable = true;
  programs.mdsh.enable = true;
  programs.nixfmt.enable = true;
  programs.shellcheck.enable = true;
  programs.shfmt.enable = true;
  programs.yamlfmt.enable = true;
  programs.just.enable = true;
  programs.gofmt.enable = true;
  programs.sql-formatter.enable = true;
  programs.sql-formatter.dialect = "postgresql";
  programs.terraform.enable = true;

}
