{
  description = "Eyot language";
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.11";
  inputs.utils.url = "github:numtide/flake-utils";
  
  outputs = { self, nixpkgs, utils }:
    utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        inputs = [
          pkgs.go
          pkgs.opencl-headers
          pkgs.ocl-icd
        ];

        makeChecks = compiler: enable_sanitiser : pkgs.stdenvNoCC.mkDerivation {
          name = "eyot-" + compiler + "-tests";
          src = ./src;
          buildInputs = inputs ++ [ pkgs.which pkgs.${compiler} ];
          thisCompiler = compiler;
          buildPhase = (if enable_sanitiser then "export EyotDebug=y\n" else "" ) + ''
            export CC=$thisCompiler
            export GOCACHE=$(pwd)/gocache
            source ${self}/contrib/env.sh
            eyot_test
          '';
          installPhase = ''
            touch $out
          '';
        };

        lib_folder =
          pkgs.stdenv.mkDerivation {
            name = "eyot-lib";
            src = ./lib;
            buildPhase = "";
            installPhase = ''
              mkdir -p $out/runtime
              cp -r ./std $out/
              cp ./runtime/eyot-runtime-* $out/runtime/
            '';
          };

        deb =
          pkgs.stdenv.mkDerivation {
            name = "eyot-deb";
            src = ./.;
            buildInputs = inputs ++ [pkgs.dpkg];
            
            buildPhase = ''
              export GOCACHE=$(pwd)/gocache
              export DpkgRoot=$(pwd)/dpkg-root

              mkdir -p $DpkgRoot
              cp -r contrib/DEBIAN $DpkgRoot/

              mkdir -p $DpkgRoot/usr/bin $DpkgRoot/usr/share
              cp -r ${lib_folder} $DpkgRoot/usr/share/eyot

              pushd src
              GOOS=linux GOARCH=amd64 go build -ldflags "-X eyot/program.EyotRoot=/usr/share/eyot" -o $DpkgRoot/usr/bin/eyot eyot/cmd
              popd

              dpkg-deb --build --root-owner-group $DpkgRoot eyot.deb
            '';

            installPhase = ''
              mkdir -p $out
              mv eyot.deb $out
            '';
          };

        # Eyot built as a nix package
        eyot_package =
          pkgs.stdenv.mkDerivation {
            name = "eyot";
            src = ./.;
            propagatedBuildInputs = [
              pkgs.gcc
            ];
            buildInputs = [
              pkgs.opencl-headers
              pkgs.ocl-icd
            ];
            nativeBuildInputs = [
              pkgs.go
              lib_folder
            ];

            doCheck = true;
            
            buildPhase = ''
              export GOCACHE=$(pwd)/gocache
              cd src
              go build -ldflags "-X eyot/program.EyotRoot=$out/lib" -o eyot eyot/cmd
            '';

            # Basic check that the binary is ok - we could run the full suite?
            checkPhase = ''
              echo "cpu fn main() { print_ln(\"hello world\") }" > minimal.ey
              EyotRoot=$(pwd)/../lib ./eyot run minimal.ey
            '';

            installPhase = ''
              mkdir -p $out/bin 
              cp -r ${lib_folder} $out/lib
              mv eyot $out/bin/
            '';
          };

        docs =
          pkgs.stdenv.mkDerivation {
            name = "eyot-docs";
            src = ./docs;
            buildInputs = [
              pkgs.mkdocs
              deb
            ];
            
            buildPhase = ''
              mkdocs build
            '';

            installPhase = ''
            mv site $out
              cp ${deb}/eyot.deb $out/installing/eyot-latest.deb
            '';
          };

        check_example = name: pkgs.stdenvNoCC.mkDerivation {
            name = "eyot-example-" + name;
            src = ./examples + "/${name}";
            buildInputs = [
              eyot_package
            ];
            
            buildPhase = ''
              echo "About to build"
              eyot build main.ey || exit 1
              echo "About to run"
              ./out.exe
            '';

            installPhase = "touch $out";

        };

      in rec {
        packages = {
          default = eyot_package;
          docs = docs;
          deb = deb;
        };

        checks = {
          clang = makeChecks "clang" false; # TODO sanitise when the bad pointer function pointer is fixed

          # builds
          build-eyot = eyot_package;
          build-deb = deb;
          build-docs = docs;

          example-hello = check_example "hello-world";

          # Check the reformat script is working
          # NB this needs to mutate the source folder so it can't use the default immutable folder
          # we must set it writeable so it doesn't fail if the code needs reformatting
          reformat = pkgs.stdenv.mkDerivation {
            name = "eyot-reformat-tests";
            src = ./src;
            buildInputs = inputs ++ [ pkgs.clang-tools_14 ];
            buildPhase = ''
              WorkingFolder=$TMPDIR/src
              mkdir -p $WorkingFolder
              cp -r ${self}/* $WorkingFolder/
              chmod -R 0777 $WorkingFolder
              cd $WorkingFolder
              source contrib/env.sh || {
                echo "Sourcing failed"
                exit 1
              }
              eyot_reformat
            '';
            installPhase = ''
              touch $out
            '';
          };
        } // (if pkgs.stdenv.isLinux then {
          # TODO all of this *should* work on macOS

          # This seems to be an issue with how gcc works with 
          gcc = makeChecks "gcc" true;

          # check that eyot_test_folder works
          check-folder = pkgs.stdenv.mkDerivation {
            name = "eyot-check-folder-tests";
            src = ./.;
            buildInputs = inputs ++ [ pkgs.gcc ];
            buildPhase = ''
              export CC=$thisCompiler
              export GOCACHE=$(pwd)/gocache
              source ${self}/contrib/env.sh
              eyot_test_folder tests/basic-language/consts
            '';
            installPhase = ''
              touch $out
            ''  ;
          };
        } else {});

        # these are shells containing eyot rather than shell for devloping eyot with
        devShells = {
          default =
            pkgs.mkShellNoCC {
              name = "eyot-shell";
              buildInputs = [
                pkgs.gcc
                eyot_package
              ];
              shellHook = "export EyotDisableCl=y";
            };
        };
      });
}
