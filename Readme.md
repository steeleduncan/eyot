[![BL tests](https://github.com/steeleduncan/eyot/actions/workflows/tests.yaml/badge.svg)](https://github.com/steeleduncan/eyot/actions/workflows/tests.yaml) [![Documentation](https://img.shields.io/badge/Documentation-blue)](https://steeleduncan.github.io/eyot) [![Playground](https://img.shields.io/badge/Documentation-blue)](https://eyot-playground.cowleyforniastudios.com)

# Eyot - A language where the GPU is just another thread

## :warning: Eyot is a work in progress. I'm develping it in the open, and it is useable for those who enjoy experimenting with new stuff, but it is unstable :warning:

Eyot is an experiment to write a language in which it is no harder to run code on the GPU than it is to run it on a background thread. 

Eyot source code is transparently compiled for both CPU and GPU, with communication between the two handled by the runtime. Traditional GPU programming expects you to handle many tasks, such as memory allocation, compiling the kernel, scheduling work, etc. These have long been handled by a language runtime when writing code for the CPU, and Eyot extends that convenience to code destined for the GPU as well. It can be thought of as an entire language built around a CUDA model of GPU concurrency.

The intended users are those in areas where the GPU or other accelerators are used heavily, e.g. game development, numerical analysis and AI.

The full documentation can be found [here](https://steeleduncan.github.io/eyot/), but to take a simple example, the following code will square a vector of numbers on the GPU

```
fn square(value i64) i64 {
   print_ln("Square: ", value)
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

You can play with this in the [Eyot Playground](https://eyot-playground.cowleyforniastudios.com#simple-gpu-usage). To break it down:

- `square` defines a function that can run anywhere, takes a 64 bit integer, prints it to the log, squares it and returns it
- `main` defines the entrypoint, a function called `main` that can only run on the CPU , denoted by the `cpu` keyword
- `let w = gpu square` uses the `gpu` keyword to create a GPU *worker* using the `square` function as a kernel
- The `send` function ships a vector of four integers to that *worker*, which logs them and squares them
- The `drain` function waits for all those values to be shipped back from the *worker* and returns them as a vector ...
- ... which are iterated over by the `for` loop and logged

The log will show first the values logged in the `square` function on the GPU (`(gpu 0) Square: 1`, `(gpu 0) Square: 2`, ...), followed by resulting values (`- 1`, `- 4`, ...).

You can swap `cpu` to `gpu` in this example, and it will do the work on a background CPU thread instead.
This is how Eyot tries to be different: the CPU and GPU are as equal as possible in terms of syntax and data model.

Eyot is ready to experiment with, and I welcome feedback from anyone who tries it.
It is very, very early stage though, and I make no promises about stability, backwards compatibility or a lack of showstopping bugs.

Finally, to preempt one question, the name has no meaning other than being a reference to Aston's Eyot, a park near where I live in Oxford.

# How to try it?

## [Eyot playground](https://eyot-playground.cowleyforniastudios.com)

[The playground](https://eyot-playground.cowleyforniastudios.com) runs the bleeding edge version of Eyot. The computing resources dedicated to it are modest though, and the GPU is virtual, so you will likely get better results running it locally.

## Installing the `.deb` (Ubuntu/Debian and their derivatives)

Eyot can be installed using `eyot-latest.deb`, downloaded from [here](eyot-latest.deb).

```
sudo apt install --reinstall --yes ./eyot-latest.deb
```

This `.deb` installs most of the required dependencies, but you will need to install the appropriate OpenCL ICD driver for your system.

## With Nix (macOS, any Linux)

If Nix is installed, and flakes are enabled, you can run an eyot file at `/path/to/file.ey` with

```
nix run github:steeleduncan/eyot -- run /path/to/file.ey
```

Please note that the flake is not setup right now to install ICDs, so it won't work with GPU threads (I'd welcome a PR from anyone who knows how to use OpenCL from within a flake).

## From source (e.g. for development)

To build Eyot on Linux or macOS you should install 

- OpenCL
- golang (>= 1.18)
- either gcc or clang

On Ubuntu or Debian this would be `sudo apt install gcc ocl-icd-opencl-dev golang -y`. 

To use the GPU on Linux you would also need the specific ICD for you GPU. For Intel GPUs this would be `sudo apt install intel-opencl-icd -y`, for Nvidia it would be `sudo apt install nvidia-opencl-icd -y`. (Please feel free to send a PR that updates this guide for other Linuxes or GPU manufacturers)

Once everything is installed, clone the repo and source the environment

```
git clone https://github.com/steeleduncan/eyot
cd eyot
source contrib/env.sh
```

And you can run an eyot file at `/path/to/file.ey` with

```
eyot run /path/to/file.ey
```

## Windows

I have tried Eyot on Windows from time to time, so it is not far from functional, but right now it is likely to be broken, and I would recommend WSL to try it out there.

# FAQs

- **Why write a new programming language for this** GPUs these days are hardly some hidden underused computing resource. This is in large part because there are already many great ways of working with the GPU. I am adding to that lengthy list of libraries and language extensions because I believe a programming language built from scratch to run on many heterogeneous processors will eventually be the least painful mode to work with GPU.

- **Who is this intended for?** Eyot would be most useful for those developing games, crunching numbers and working on AI. Essentially anyone who wants to use both the CPU and GPU efficiently, without the hassle of doing so directly

- **What stage is it at?** This is very early and experimental. The compiler is quite inefficient, with poor error messages, and backends onto your system's C compiler. The language itself is subject to change

- **What guarantees do I have regarding language stability?** For now, none. Eyot is experimental, and things will change and break. That said, Eyot is written in Go with no dependencies. If you do decide to use it, clone the repo, keep a note of the commit hash, and you can pin your project to an old version of Eyot indefinitely

- **Does it have language feature X?** No/almost certainly not (yet). The language is as minimal as possible right now to avoid weighing down the compiler, and keeping it easy to experiment with the CPU/GPU interface. Once that is working well I can turn to these features

- **How efficient is Eyot?** If you are running an embarrasingly parallel number crunching problem on your CPU, then switching to Eyot and moving the computation to your GPU may already win you some time. However Eyot's runtime is far from optimised right now. If you are using a highly optimised GPGPU library, and switch to Eyot, then it is unlikely Eyot will compete (yet). With work, and a decent standard library, I'd like to close that gap in the future though!

- **Why does it ask for a C compiler?** For now Eyot transpiles to C, and calls the C compiler to build native code on that platform

- **Will it always be able output to C?** Yes. My hope is for a future version of Eyot to output SPIR-V and native code directly, but the ANSI C backend will stay. Game developers ship code on a lot of platforms, and many of these platforms have NDAs that hamper porting and shipping an open source project on them. It is hard to imagine a platform where you can't compile and run ANSI C though

- **Why aren't there more tests of the Go code** Although there are some go tests, the majority of the tests are integration tests compiling & running Eyot code. I intend to port the compiler from Go to Eyot once it is more settled, and unlike with Go tests, the work put into the integration tests will continue to be valid after porting

- **Why Eyot?** It is named for Aston's Eyot, a small park near where I live in Oxford, there is no meaning beyond that

# Non-goals

I wouldn't want anyone to spend time learning Eyot and be disappointed, so some non-goals:

- **Be the next great CPU-side language** I want as few syntax differences between GPU and CPU as possible, so language design is bound by the GPU capabilities. This restricts what I can add to Eyot's syntax, and if you are looking to write CPU-side code only, there are likely to be more featureful languages out there for you.

- **Automatic parallelisation** Eyot does not automatically parallelise work across CPU/GPU cores, instead it makes it easy to write code that does parallelise work across your CPU/GPU cores. The intention is to give a programmer a convenient option for distributing work across processors, not reduce the programmer's control.

- **Theoretically optimal performance** Eyot is not intended as a total replacement for current GPGPU libraries any more than C and C++ are intended as a total replacements for Assembly. For me ease of use is an acceptable price to pay for some performance penalties. That said, I would consider significant performance deviations between Eyot code and equivalent C/Vulkan code to be a bug.

# Roadmap

Eyot is very unfinished. In general my focus has been to experiment with the CPU/GPU split, and keep everything else as straightforward as possible for now, but now that is settling, my rough roadmap is

- **Syntax** The current syntax is a minimal, hopefully familiar, modernish *C with classes* type of language. I wanted something easy to learn & implement on both the CPU & GPU. I plan to add Lambdas, Algebraic Data Types, Operator overloading and Reflection

- **Memory management** The memory model is not final. Sharing data between GPU and CPU has particular difficulties that will motivate the eventual form of memory management used in Eyot. For now Eyot has a rudimentary Garbage Collector, and my intention is to extend it such that data is transparently moved between CPU memory and the different forms of GPU memory (using shared memory where that exists)

- **Rendering support** Currently Eyot only allows use of the GPU for calculations. I'm working to expand it to support rendering as well.

- **Standard library** Standard libraries are both a utility for users, and extended documentation for a language. I would like to develop Eyot's, but for now it is limited to a small number of utilities I've required for testing.

- **CPU threading model** Each CPU worker is a CPU thread right now. This is not efficient if you are looking to spawn many workers in a lightweight manner. Also, when calling a blocking keyword such as `drain`, it would make sense to yield execution of a coroutine. Overall, I think coroutine support with an M:N scheduler onto OS threads would be a useful project that would not change the syntax.

- **Cores that aren't CPU or GPU** There are many other (co)processors Eyot could help you take advantage of (e.g. NPU, FPGA), and eventually I would like it to.

- **The current compiler** This compiler is a stopgap. I've chopped and changed its codebase regularly when developing Eyot, and it shows. At some point when Eyot's design is more settled I will rewrite it in Eyot. Go was chosen as it is compiles quickly, cross compiles easily, and the language is reasonably minimal so it should translate easily to Eyot

- **Backend library** OpenCL is straightforward, platform independent, commonly supported and works with all major GPU vendors. I suspect it is not the optimal choice, so I have kept it easy to add alternate backends, and I would like to experiment with Metal, Vulkan, and D3D compute backends in the future

