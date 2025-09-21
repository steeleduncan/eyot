{
  description = "Simple example of consuming the eyot flake";
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.11";
  inputs.utils.url = "github:numtide/flake-utils";
  inputs.eyot.url = "github:steeleduncan/eyot";
  
  outputs = { self, nixpkgs, utils, eyot }:
    utils.lib.eachDefaultSystem (system:
    let
      pkgs = nixpkgs.legacyPackages.${system};
      
    in rec {
      packages = {
        default =
          pkgs.stdenv.mkDerivation {
            name = "hello-eyot";
            buildInputs = [
              pkgs.go
              eyot.packages.${system}.default
            ];
            src = ./.;
            buildPhase = ''
              eyot build main.ey
            '';

            installPhase = ''
              mkdir -p $out/bin
              mv out.exe $out/bin/
            '';
          };
      };
    });
}

