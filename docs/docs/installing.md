# Installing

Eyot can be installed in a few ways right now

## Debian derivatives (e.g. Ubuntu)

Eyot can be installed on Debian, Ubuntu and any derivatives via a `.deb` file. Download `eyot.deb` from [here](eyot-latest.deb).

```
sudo apt install --reinstall --yes eyot.deb
```

This will also install any dependencies Eyot requires (`opencl`, etc) and it is the recommended approach for now.

The `.deb` at that link is built along with this documentation after each successful test run, so you can re-download it and update from time to time.

## Nix

Running `nix build github:steeleduncan/eyot` will leave an Eyot installation in the `result` folder.

You can also add the flake as an input to your own, and use Eyot in that manner.
