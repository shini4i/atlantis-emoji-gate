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
          pkgs.delve
          pkgs.golangci-lint
          pkgs.gitleaks
          pkgs.go-junit-report
          pkgs.mockgen
        ];
      };

      packages.${system}.default = pkgs.buildGoModule {
        pname = "atlantis-emoji-gate";
        version = "0.4.0";
        src = ./.;
        subPackages = [ "cmd/emoji-gate" ];
        vendorHash = "sha256-fBzZxngam2jqB//4rCkKg69Yv0eiz54wu0CQhvlm4xs=";

        # Generated mocks are not checked into git. During the module
        # download phase Nix still resolves all imports (including test
        # files), so we create minimal stubs so the packages exist.
        overrideModAttrs = old: {
          preBuild = ''
            mkdir -p internal/client/mocks internal/processor/mocks
            echo 'package mocks' > internal/client/mocks/mock_client.go
            echo 'package mocks' > internal/processor/mocks/mock_processor.go
          '';
        };
      };
    };
}
