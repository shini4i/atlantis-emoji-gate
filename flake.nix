{
  description = "A development environment for the atlantis-emoji-gate Go project";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
    in
    {
      devShells.${system}.default = pkgs.mkShell {
        buildInputs = with pkgs; [
          go_1_25
          gopls
          delve
          go-task
          golangci-lint
          gosec
          govulncheck
          gotools # goimports, used by the pre-commit go-imports hook
          gocyclo
          gitleaks
        ];
      };
    };
}
