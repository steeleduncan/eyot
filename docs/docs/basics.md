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

## Types

### Integers

The only integer type so far is `i64`, which denotes a 64 bit signed integer.

### Float

Currently there are 32 and 64 bit floating point types `f32` and `f64` respectively.
Due to limitations in the standard profile of OpenCL, only 32 bit floats are allowed in GPU code.
I hope to lift this restriction by moving to a different back end on day.

`1.0` is an `f64` literal, and `1.0f` is an `f32` literal.

### Boolean

`bool` is a boolean type, of which `true` and `false` represent the only two values.

Eyot has the usual numerical comparison operators (`==`, `!=`, `<`, `>`, `<=`, `>=`) , all of which return `bool`.

`bool` values can be negated with the `not` keyword 

### Vector

A vector (flexible array) of values is denoted by square brackets around the type it contains.
E.g. `[i64]`.
Literals are denoted by braces after the type

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

### Strings and Characters

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

The access operator, `[]`, takes the code point index as a value, and returns a `char` type, which is a type sufficient to represent all Unicode code points

### Casting

You can cast with the `as` keyword. For example `let u = 1` would declare u to be a variable of type `i64`, however `let u = 1 as f64` would declare u to be a variable of type `f64`.
