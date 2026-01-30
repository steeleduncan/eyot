# Installing

Linux and macOS users can try Eyot in a few ways right now. Windows support is on the way, but for now Windows users would be best advised to use WSL and follow the instructions for Debian derivatives/

## Development environment

If you clone the repository, and run

```
source contrib/env.sh
```

from the root, it places an `eyot` command in your path that functions exactly like the binary would. It is the easiest way to try out for most users as it installs nothing, but you require

- **bash** This may work against other shells, but it is only tested against `bash` for now
- **go** Any version >= 1.18 should be fine
- **Clang/GCC** It is only explicitly tested against the latest Clang/GCC versions, but any C compiler that handles C99 with standard command line options should work

## Debian derivatives (e.g. Ubuntu)

Eyot can be installed on Debian, Ubuntu and any derivatives via a `.deb` file. Download `eyot-latest.deb` from [here](eyot-latest.deb).

```
sudo apt install --reinstall --yes ./eyot-latest.deb
```

This will also install any dependencies Eyot requires (`opencl`, `gcc` etc). This `.deb` is only updated if all tests pass, so it is the most likely option to be functional right now.

## Nix

Run `nix run github:steeleduncan/eyot run /path/to/file.ey` to build and execute the specified Eyot file using the bleeding edge version of Eyot.

There is an example of consuming eyot from a flake in [examples/hello-world](https://github.com/steeleduncan/eyot/tree/main/examples/hello-world).
