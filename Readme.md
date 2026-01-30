[![BL tests](https://github.com/steeleduncan/eyot/actions/workflows/tests.yaml/badge.svg)](https://github.com/steeleduncan/eyot/actions/workflows/tests.yaml)

# Eyot - A language where the GPU is just another thread

## :warning: Eyot is a work in progress. I'm develping it in the open, and it is useable for those who enjoy experimenting with new stuff, but it is unstable :warning:

Eyot is an experiment to write a language in which it is no harder to dispatch a task to the GPU than it is to run a task on a background thread. All aspects of the Eyot's design are directed towards this goal. It can be thought of as an entire language built around a CUDA model of GPU concurrency.

The full documentation can be found [here](https://steeleduncan.github.io/eyot/), but to take a simple example, the following code will square a vector of numbers on the GPU

```
fn square(value i64) i64 {
   return value * value
}

cpu fn main() {
    let w = gpu square
    send(w, [i64]{ 1, 2, 3, 4 })
    for v: drain(w) {
        print_ln("- ", v)
    }
}
```

Breaking this down, we start with the minimal hello world example

```
cpu fn main() {
    print_ln("Hello World!")
}
```

The `cpu` keyword indicates that this function can only run on the CPU. This unlocks certain CPU specific capabilities later. This can be extended with a function to square a number 

```
fn square(value i64) i64 {
   print_ln("Squaring ", value)
   return value * value
}

cpu fn main() {
    print_ln("2 squared = ", square(2))
}
```

The `square` function does not need the `cpu` keyword as it will use no CPU dependent code. To iterate over a vector of numbers and print them, we use a vector literal and a `for` loop

```
fn square(value i64) i64 {
   print_ln("Squaring ", value)
   return value * value
}

cpu fn main() {
    let values = [i64]{ 1, 2, 3, 4 }
    for x: values {
        let y = square(x)
        print_ln("- ", y)
    }
}
```

This work can be parallelised over multiple cpu threads by using the `cpu` keyword to create a CPU-side *worker* out of the `square` function. `send` will ship values to it, and `drain` will wait for the worker to finish and pull all values back.

```
fn square(value i64) i64 {
   print_ln("Squaring ", value)
   return value * value
}

cpu fn main() {
    let w = cpu square
    send(w, [i64]{ 1, 2, 3, 4 })
    for v: drain(w) {
        print_ln("- ", v)
    }
}
```

Finally if you swap `cpu` to `gpu` in this example it will ship that work to your GPU instead.
Any `print_ln` logs generated on the GPU will be printed as the values return to the CPU side code.

This is how Eyot tries to be different: the CPU and GPU are as equal as possible in terms of syntax and data model.
Shifting a calculation to the GPU is often as trival as swapping a single character in the source code.

Eyot is ready to experiment with, and I welcome feedback from anyone who tries it.
It is very, very early stage though, and I make no promises about stability, backwards compatibility or a lack of showstopping bugs.

Finally, to preempt one question, the name has no meaning other than being a reference to Aston's Eyot, a park near where I live in Oxford.

# How to try it?

Currently it runs on Linux and macOS. Windows is not far from functional.

## Using the `.deb` (Ubuntu/Debian and their derivatives)

Eyot can be installed using `eyot-latest.deb`, downloaded from [here](eyot-latest.deb).

```
sudo apt install --reinstall --yes ./eyot-latest.deb
```

This installs dependencies, and it is only updated when all tests pass, so it is the most likely option to be functional right now.

## With Nix

If Nix is installed and flakes are enabled you can run an eyot file at `/path/to/file.ey` with

```
nix run github:steeleduncan/eyot -- run /path/to/file.ey
```

Please note that the flake is not setup right now to install ICDs, so it won't work with GPU threads (I'd welcome a pull request from anyone who knows how to use OpenCL from within a flake).



## From source (e.g. for development)

To run Eyot on a linux system you would need to install OpenCL, golang (>= 1.18) and either gcc or clang. On Ubuntu or Debian this would be `sudo apt install gcc ocl-icd-opencl-dev golang -y`. Likely you would also need the specific ICD for you GPU. For intel GPUs this would be `sudo apt install intel-opencl-icd -y` and for Nvidia replace `sudo apt install nvidia-opencl-icd -y`. (Please feel free to update this guide for other Linuxes or GPU manufacturers)

Once that is done you can source the environment

```
git clone https://github.com/steeleduncan/eyot
cd eyot
source contrib/env.sh
```

Then run you can run an eyot file at `/path/to/file.ey` with

```
eyot run /path/to/file.ey
```

# Why write a programming language for this?

There are

- **Great libraries for offloading specific operations to GPUs** (e.g. pytorch), but they are limited to the functionalities that library supports
- **APIs for interfacing with the GPU** (e.g. OpenCL, Vulkan compute), but these are intricate low level libraries, and although they make it possible to use your GPU for general computation, they don't make it easy
- **Language modifications such as CUDA or Triton**, which offer a more integrated approach, but they are additions to existing languages with all the compromises that entails, and not always portable

Eyot is designed from scratch to run on many heterogeneous processors.
Everything about it, from syntax to memory model, is chosen towards that goal, and my hope is that by designing it in this way it will be

# Non-goals

I wouldn't want anyone to spend time learning Eyot and be disappointed, so some non-goals:

- **Be the next great CPUside language** I want as few syntax differences between GPU and CPU as possible, so language design is bound by the GPU capabilities, which restricts what I can add to Eyot's syntax. As a result, although Eyot is useable as a CPUside language, it shines when running code across GPUs and CPUs, and if you are looking to write CPU-side code only, there are likely to be more featureful options for you.

- **Automatic parallelisation** Eyot does not automatically parallelise work across CPU/GPU cores, rather it makes it easy to write code that does parallelise work across your CPU/GPU cores. The intention is to give a programmer a convenient option for distributing work across processors, not reduce their control.

- **Theoretically optimal performance** Eyot is not intended as a total replacement for current GPGPU libraries any more than C and C++ are intended as a total replacements for Assembly. For me ease of use is an acceptable price to pay for some performance penalties. That said, I would consider significant performance deviations between Eyot code and equivalent C/Vulkan code to be a bug.

# Roadmap

Eyot is very unfinished. In general my focus has been to experiment with the CPU/GPU split, and keep everything else as straightforward as possible for now, but my broad roadmap is

- **Syntax** The current syntax is a minimal, hopefully familiar, modernish *C with classes* type of language. I wanted something easy to learn & implement on both the CPU & GPU. I plan to add Lambdas, Algebraic Data Types, Operator overloading and Reflection

- **Memory management** The memory model is not final. Sharing data between GPU and CPU has particular difficulties that will motivate the eventual form of memory management used in Eyot. For now Eyot has a rudimentary Garbage Collector, and my intention is to extend it such that data is transparently moved between CPU memory and the different forms of GPU memory (using shared memory where that exists)

- **Rendering support** Currently Eyot only allows use of the GPU for calculations. I'm working to expand it to support rendering as well.

- **Standard library** Standard libraries are both a utility and extended documentation for a language. I have not started on Eyot's standard library in earnest.

- **Threading model** Each CPU worker is a CPU thread right now. This is not efficient if you are looking to spawn many workers in a lightweight manner. Also, when calling a blocking keyword such as `drain`, it would make sense to yield execution of a coroutine. Overall, I think coroutine support with an M:N scheduler onto OS threads would be a useful project. This would not change the syntax of Eyot.

- **Cores that aren't CPU or GPU** The CPU and GPU are not the end of the story, there are many other (co)processors Eyot could help you take advantage of, and eventually it will, but I've not worked on that yet.

- **The current compiler** This compiler is a stopgap. I've chopped and changed its codebase regularly when developing Eyot, and it shows. At some point when Eyot's design is more settled I will rewrite it in Eyot. Go was chosen as it is quick to (cross-) compile and run

- **Backend library** OpenCL is straightforward, platform independent, commonly supported and works with all major GPU vendors, so I have started with it. It should not be difficult to add alternate backends though, and I would like to add Metal compute, Vulkan compute, and D3D compute backends in the future

# FAQs

- **Who is this intended for?** Eyot would be most useful for those developing games, crunching numbers and working on AI. Essentially anyone who wants to use both the CPU and GPU efficiently, without the hassle of doing so directly

- **What stage is it at?** This is very early and experimental. The compiler is quite inefficient, with poor error messages, and backends onto your system's C compiler. The language itself is subject to change

- **What guarantees do I have regarding language stability?** For now, none. Eyot is experimental, and things will change and break. That said, Eyot is written in Go with no dependencies. If you do decide to use it, clone the repo, keep a note of the commit number, and you can pin your project to an old version of Eyot indefinitely

- **Does it have language feature X?** No/almost certainly not (yet). The language is as minimal as possible right now to avoid weighing down the compiler, and keeping it easy to experiment with the CPU/GPU interface. Once that is working well I can turn to these features

- **How efficient is Eyot?** If you are running an embarrasingly parallel number crunching problem on your CPU, then switching to Eyot and moving the computation to your GPU may already win you some time. However Eyot's runtime is far from optimised right now. If you are using a highly optimised GPGPU library, and switch to Eyot, then it is unlikely Eyot will compete yet. With work, and a decent standard library, I'd like to close that gap in the future though!

- **Why does it ask for a C compiler?** For now Eyot transpiles to C, and calls the C compiler for you. Without a C compiler on your system, it won't work

- **Will it always be able output to C?** Yes. My hope is for a future version of Eyot to output SPIR-V and native code directly, but the ANSI C backend will stay. Game developers ship code on a lot of platforms, and many of these platforms which have NDAs that hamper porting and shipping an open source project on them. It is hard to imagine a platform where you can't compile and run ANSI C though

- **Why aren't there more tests of the Go code** Although there are some go tests, the majority of the tests are integration tests compiling & running Eyot code. I intend to port the compiler from Go to Eyot once it is more settled, and unlike with Go tests, the work put into the integration tests will continue to be valid after porting

- **Why Eyot?** It is named for Aston's Eyot, a small park near where I live in Oxford, there is no meaning beyond that
