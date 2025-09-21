These are the sources for libraries depended on by Eyot binaries

- `runtime`: Thes are the C sources for the Eyot runtime, both CPU-side utilities like the garbage collector, GPU utilities, and the interface code
- `std`: This is the Eyot std library. The majority of this is written in Eyot, but there are also `.c` files where appropriate, and the `.json` files that instruct Eyot which `C` functions should be exposed to Eyot code
