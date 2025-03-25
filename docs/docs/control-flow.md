# Control flow

## If

Eyot's `if` is similar to C's, but without the braces.

```
if x > 1 {
    ...
}
```

It must be given a boolean value as there are no type coercions. E.g. You must write

```
if x.length() != 0 {
    ...
}
```

rather than the common shorthand a C programmer may be used to:

```
if x.length() {
    ...
}
```

## While

While is just like an if statement, but rather than execute once *if* the condition is true, it will execute continuously *while* the condition is true

```
while queue.has_message() {
    let msg = queue.get_message()
    ...
}
```

## Foreach

In Eyot you can iterate over vectors with `foreach`

```
let xs = [i64] { 6, 4, 7, 3 }
foreach x: xs {
    print_ln("x = ", x)
}
```

Iterating over a range is the same, but using the `range` builtin

```
foreach x: range(5) {
    print_ln("x = ", x)
}
```

is equivalent to

```
let xs = [i64] { 0, 1, 2, 3, 4 }
foreach x: xs {
    print_ln("x = ", x)
}
```

The `range` builtin has type `[i64]` and generates valid integer vector literals that can be used like any other vector literal.
However there is a special case in the compiler to expand the iteration in `foreach x: range(5)` to a simple and efficent loop rather than heap allocating a vector.
It is recommended to write it that way, rather than assigning `range(5)` to a separate variable, if you can.

This leads to a edge case in which `range` cannot be used in GPU code outside a `foreach` loop because vectors are not supported GPUside, but it can be used inside a `foreach` because no vector would be created.

