# Modules

- folders to organise modules
- `import std.encoding` pulls in members of that namespace as `encoding`

Current root namespaces

- `std` will be kept as stable as is reasonably possible up until 1.0, and not change after. This is for core standard library functionality of Eyot
- `stdx` is an experimental variant of `std`. APIs can potentially change with each version of Eyot while they are in here, but the intention is to stabilise them over time and move them to `std`

Currently there is no pinning system, but obviously that is a long term goal
