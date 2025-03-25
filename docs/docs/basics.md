# Basics

Overall Eyot's syntax should be familiar to someone used to the broad family of languages inheriting syntax from C (e.g. C++, C#, Go, Java, Rust).
I've tried to keep the syntax minimal, and unsurprising, to someone familiar with some language in that family.
This makes Eyot easy to implement and learn, and constrains my experimentations to Eyot's distinguishing feature - the way it allows you to interact with the GPU using *Workers*.

## Variables

Mutable variables can be assigned with let. The type is inferred

```
let x = 12
print_ln(x)
```

Please note that `print_ln` is a builtin function for debug printing while developing Eyot.
It is unlikely to go anywhere in the near future, but it will eventually move to the standard library.

Global variables are disallowed for now as they are complex to share with the GPU.

## Integers

The only integer type so far is `i64`, which denotes a 64 bit signed integer.

## Float

The only floating point type so far is `f64`, which denotes a 64 bit floating point number.

## Boolean

`bool` is a boolean type, of which `true` and `false` represent the only two values.

Eyot has the usual numerical comparison operators (`==`, `!=`, `<`, `>`, `<=`, `>=`) , all of which return `bool`.

`bool` values can be negated with the `not` keyword 

## Vector

A vector (flexible array) of values is denoted by square brackets around the type it contains.
E.g. `[i64]`.
Literals can be assigned with braces after the vector

```
let x = [i64] { 1, 2, 3, 4 }
```

Values can be read and set using square brackets

```
x[3] = 5
print_ln(x[3])
```

Additionally vectors support the following builtin functions
- `x.length()` returns the number of elements
- `x.resize(20)` resizes the vector to have space for `20` slots
- `x.append(1)` would add a new value to the vector

## String

String literals are created with quotes

```
let species_name = "dog"
```

They can be concatenated with the `+` function

```
let concatenated = "x" + "y"
print_ln(concatenated)
```

And similar to vectors they support `.length()` and the access operator (`[]`).
Please note that these operators work in terms of Unicode Scalar Values.
For now the details of how they may be stored are opaque to users of Eyot, and the runtime is free to chose.

