# Workers

The fundamental threading primitive is a *worker*.
All workers are created from a function, and they run on either GPU or CPU, applying that function to data passed to them.
If the function returns that data can be read back from the worker.

A simple example of logging in a background CPU thread is

```
cpu fn do_log(str string) {
	print_ln("log: ", str)
}

let w = cpu do_log

send(w, [string] { "line 1" })
send(w, [string] { "line 2" })
```

Here the `cpu` keyword is used to convert the function to a CPU-side worker, and `send` is a special function that ships data to the worker.
In cases like this where the function has no parameters it will act very similar to a normal thread creation, dispatching the function in the background.

However if the function returns data it can be read back from the worker.

```
fn square(val i64) i64 {
	return val * val
}

cpu fn main() {
	let w = cpu square
    
    send(w, [i64] { 1, 2, 3 })

	print_ln(receive(w))
	print_ln(receive(w))
	print_ln(receive(w))
}
```

Or if you want to wait for all data to be processed, you can `drain` the worker.

```
fn square(val i64) i64 {
	return val * val
}

cpu fn main() {
	let w = cpu square
    
    send(w, [i64] { 1, 2, 3 })
    
    let rets = drain(w)
	print_ln(rets[0])
	print_ln(rets[1])
	print_ln(rets[2])
}
```

or more idiomatically

```
fn square(val i64) i64 {
	return val * val
}

cpu fn main() {
	let w = cpu square
    
    send(w, [i64] { 1, 2, 3 })
    for ret: drain(w) {
        print_ln(" ", ret)
    }
}
```

Please note that workers are non-blocking, they are a threading primitive, not a synchronisation primitive.

Channels can be created GPU-side with the `gpu` primitive, to dispatch the above on the GPU is a trivial change, and perhaps the most distinguishing aspect of Eyot's design


```
fn square(val i64) i64 {
	return val * val
}

cpu fn main() {
	let w = gpu square
    
    send(w, [i64] { 1, 2, 3 })
    for ret: drain(w) {
        print_ln(" ", ret)
    }
}
```

Please note that although any function can be passed to the `cpu` keyword for conversion to a CPU-side worker, only location independent functions (those not tagged with `cpu`) can be passed to the `gpu` keyword for conversion to a GPU-side worker.

Not all work is done on vectors of data with no other parameters of course, often you pass uniform parameters to GPU kernels.
Worker functions in Eyot need to take a single parameter, which becomes the input type of the worker.
Structs can be passed of course, but would not be an idiomatic way of passing constant parameters to a worker.

A better solution in Eyot is the partial application of functions.
For example in the following you partially apply the two parameter function `multiply`, converting it to the single parameter `triple` function, and then send it values.

```
fn multiply(lhs, rhs i64) i64 {
	return lhs * rhs
}

cpu fn main() {
    let triple = partial multiply(_, 3)
	let w = gpu triple
    
    send(w, [i64] { 1, 2, 3 })
    for ret: drain(w) {
        print_ln(" ", ret)
    }
}
```

This is a lot more convenient than passing a struct in for the purpose of providing context to a function. 

