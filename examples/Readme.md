# Example code

These are some examples of Eyot in use. The tests check that these files build and run without errors, but they do not verify the output is correct.

- **hello-world** A minimal hello world example for Eyot. Run `eyot run main.ey` in the folder.
- **via-flake** An example of how to use the version of Eyot via a Nix flake. Run `nix build . && ./result/bin/out.exe`. I've left out `flake.lock` so this runs against the latest version of Eyot.
