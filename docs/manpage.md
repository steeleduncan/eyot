---
title: EYOT
section: 1
header: User Manual
footer: eyot
date: May 22, 2025
---

# NAME
**eyot** - A programming language where the GPU is just another thread

# SYNOPSIS
**eyot** <*command*> <*file*>

# DESCRIPTION
**eyot** is an experiment to write a language in which it is no harder to dispatch a task to the GPU than it is to run a task on a background thread. All aspects of the Eyot's design are directed towards this goal. It can be thought of as an entire language built around a CUDA model of GPU concurrency.

# COMMANDS
- *env* Print environment
- *build* Build the program to an executable file
- *run* Build and run a file directly
- *dump* Create the folder of runtime code as required to compile
- *lint* Lint the file, this prepares it fully for compilation, but does nothing
- *c* Output the C code (one file)

# FLAGS
- *-showlog* Show the compiler output (error or no error)

# ENVIRONMENT

- *EyotRoot* the root of the eyot runtime libraries
- *EyotTestOclGrind* if 'y' it will use oclgrind
- *CC* the C compiler to use for the backend code generation (Linux and macOS)
