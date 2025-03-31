# Functions

Functions are declared using the `fn` keyword, with arguments declared in parentheses after the name, and the return type after that.
Values can be returned with `return`.

```
fn square(val i64) i64 {
    return val * val
}
```

This and the return type can be omitted for void functions

```
fn do_something(val i64) {
    ...
}
```

## Location modifiers

The guiding goal of Eyot is a language with a single syntax that runs on all cores of your system.
Sadly this isn't always totally possible and certain code and constructs can only run on the CPU (memory allocation, worker creation, etc).
When that is the case a function must be tagged with the `cpu` keyword in its definition

For example a function to square a number is location independent

```
fn square(val i64) i64 {
    return val * val
}
```

However some operations, like allocating a vector, require the CPU. Any function containing this must be tagged with `cpu`.

```
cpu fn log_square(val i64) {
    let values = [i64] { 0, 1, 2, 3, 4 }
    ...
}
```

Calling `cpu` code from code not marked `cpu` is a compile time error.
As Eyot develops the intention is to loosen these restrictions as far as possible, but it is likely that some form of this system will always be required.

The `cpu` requirement goes as far as being part of the type signature because it is important to never assign to a variable of function type capable of running anywhere, a function that is only capable of running on the CPU, or the runtime would have to panic.
It is analogous to how you can assign a non const pointer to a const pointer in C, but not the other way around.

## Partial application

Functions can be partially applied in Eyot. For example the following function multiplies two numbers

```
fn multiply(lhs, rhs i64) i64 {
   return lhs * rhs 
}

```

If you want to turn this into a function that doubles a number you can do so as follows

```
let double_number = partial multiply(_, 2) 
print_ln(double_number(4))
```

Would print `8`.

This is a restricted version of more general closures, and it will be useful later when passing global state to workers.
Currently general closures are not part of Eyot because they can implicitly capture a great deal of state, which might then need to be shipped to the GPU, and unwittingly cause significant performance issues.
This will likely change in the future, but for now state capture must be explicit in Eyot.

This applies of course to global mutable variables, the most dangerous implicit state capture of all, which do not exist in Eyot for now.

