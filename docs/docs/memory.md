# Memory

Eyot is currently garbage collected.
I'm not keen to write a language with purely manual, and unsafe, memory management, especially given the extra difficulties of managing memory between GPU and CPU.

It is not obvious how to ensure safety at compile time the way Rust does, given the complexities of ensuring that data lifetimes persist across GPU invocations.
The different memory buffer types used by GPUs and CPUs present further complications.
Nevertheless it would be interesting one day to explore if safe static memory management in Eyot is possible.

Garbage collection seems a reasonable solution for Eyot.
The runtime can track pointers as they move from the CPU to the GPU, and back.
The runtime can also manage the memory type(s) attached to a buffer (in the cases the hardware makes that distinction).
In general it seems to allow the programmer to work away, ignorant of these problems, without any showstopping downsides.

An object might be stack allocated as follows

```
let r = Junk { x: 0 }

```

It would be of type `Junk` and passed by value to any function.
The same object could be heap allocated as a garbage collected pointer using the `new` keyword

```
let r = new Junk { x: 0 }

```
It would be of type `*Junk` and passed by reference to any function.
Eventually it would be deallocated on a run of the garbage collector once there are no references left to it.

