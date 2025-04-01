# Modules

Modules are organised along the lines of folders. You can import a module with `import`

```
import foo::bar
```

would import `foo/bar.ey`. There is currently no visibility system for modules, so all symbols from `bar.ey` are imported and added to the `foo::bar` namespace. For example if you had a function `square` in `bar.ey` you would refer to it as `bar::square`

## Convention

Some conventions with the standard library rather:

- `std` will be kept as stable as is reasonably possible up until 1.0, and not change after. This is for core standard library functionality of Eyot
- `stdx` is an experimental variant of `std`. APIs can potentially change with each version of Eyot while they are in here, but the intention is to stabilise them over time and move them to `std`
