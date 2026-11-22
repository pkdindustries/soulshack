{
  description = "soulshack: an IRC chatbot";

  # the 26.11 pre-release; nixos-26.11 once that branch exists
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        # development on a Mac
        "aarch64-darwin"
      ];
    in
    {
      packages = nixpkgs.lib.genAttrs systems (
        system:
        let
          soulshack = nixpkgs.legacyPackages.${system}.callPackage ./nix/package.nix {
            rev = self.shortRev or self.dirtyShortRev or "dirty";
          };
        in
        {
          inherit soulshack;
          default = soulshack;
        }
      );

      # `nix develop`: the go toolchain, and what the shell tools call.
      devShells = nixpkgs.lib.genAttrs systems (system: {
        default = nixpkgs.legacyPackages.${system}.mkShell {
          packages = with nixpkgs.legacyPackages.${system}; [
            go
            jq
            curl
          ];
        };
      });
    };
}
