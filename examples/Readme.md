# Example code

These are some examples of Eyot in use, unless stated otherwise run `eyot run main.ey`. Any example with a `description.txt` file will be included in the Eyot playground

- **hello-world** A minimal hello world example for Eyot
- **simple-gpu-usage** Square a vector of numbers on the GPU
- **partial-application** An example of partial function application, which is useful for passing context information to GPU kernels
- **backpropagation** A simple neural network written in Eyot
- **consume-flake** An example of how to use the version of Eyot via a Nix flake. Run `nix build . && ./result/bin/out.exe`. I've left out `flake.lock` so this runs against the latest version of Eyot.

The tests check that these files build and run without errors, but they do not verify the output is correct.
