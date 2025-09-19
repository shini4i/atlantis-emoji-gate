{
  description = "A development environment for the atlantis-emoji-gate Go project";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.05";
  };

  outputs = { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};

      goToolchain = pkgs.go_1_24;

    in
    {
      devShells.${system}.default = pkgs.mkShell {
        buildInputs = [
          goToolchain
          pkgs.gopls
          pkgs.golangci-lint
          pkgs.gitleaks
          pkgs.go-junit-report
        ];
      };

      packages.${system}.default = pkgs.buildGoModule {
        pname = "atlantis-emoji-gate";
        version = "0.4.0";
        src = ./.;
        vendorHash = "sha256-OVEY4TPnZ2Sq4wc8Zg6zhnRms8Jm1cfZK1EzzOWUasM=";
      };
    };
}
