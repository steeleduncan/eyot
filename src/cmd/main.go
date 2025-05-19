package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"eyot/errors"
	"eyot/output/crunner"
	"eyot/output/cwriter"
	"eyot/output/textwriter"
	"eyot/program"
)

func usage() *errors.Errors {
	fmt.Print(`eyot <command> <file>
  Commands:
    env
      Print environment

    build
      Build the program to an executable file

    run
      Run directly

    dump
      Create the folder of runtime code as required to compile

    lint
      Lint the file, this prepares it fully for compilation, but does nothing

    c
      Output the C code (one file)

  Flags:
    -showlog
      Show the compiler output (error or no error)

  EnvironmentVariables:
    EyotRoot: the root of the eyot runtime libraries
    EyotTestOclGrind: if 'y' it will use oclgrind
    CC: the C compiler to use for the backend code generation (Linux and macOS)
`)
	return nil
}

const (
	// print the relevant file
	kPrint int = iota

	// silently process the file (only relevant for debugging)
	kSilent

	// create a folder with all sources
	kPrepare

	// compile the folder
	kCompile

	// compile and run
	kRun

	// print env
	kEnv
)

// parse options and invoke doCompile
func errMain() *errors.Errors {
	action := kCompile
	outFile := "./out.exe"
	filePath := ""

	flags := map[string]bool{}
	args := []string{}

	useOclGrind := os.Getenv("EyotTestOclGrind") == "y"

	for _, arg := range os.Args {
		if len(arg) == 0 {
			continue
		}

		if strings.HasPrefix(arg, "-") {
			flags[arg[1:]] = true
		} else {
			args = append(args, arg)
		}
	}

	if len(args) == 2 && args[1] == "env" {
		action = kEnv
	} else {
		if len(args) != 3 {
			return usage()
		}

		filePath = args[2]
		switch args[1] {
		case "build":
			action = kCompile

		case "c":
			action = kPrint

		case "lint":
			action = kSilent

		case "dump":
			action = kPrepare

		case "run":
			action = kRun
		}
	}

	env := program.CreateEnvironment(filepath.Dir(filePath))
	es := errors.NewErrors()
	if action == kEnv {
		fmt.Println("EyotRoot")
		for _, root := range env.Roots {
			fmt.Println("  " + root)
		}
		return es
	}

	if filepath.Ext(filePath) != ".ey" {
		es.LogInternalError(fmt.Errorf("Bad extension: %v", filePath))
	}

	var outStream io.Writer = os.Stdout

	pid := os.Getpid()

	buildRootDir := filepath.Join(os.TempDir(), fmt.Sprintf("eyot-build-root-%v", pid))
	if action != kPrepare {
		defer os.RemoveAll(buildRootDir)
	}

	var cr crunner.CRunner = nil

	if action == kRun {
		outFile = filepath.Join(buildRootDir, fmt.Sprintf("temp-binary-%v.exe", os.Getpid()))
		defer os.Remove(outFile)
	}

	if !(action == kPrint || action == kSilent) {
		os.MkdirAll(buildRootDir, 0777)
		files := cwriter.DumpRuntime(buildRootDir, env)
		cr = crunner.NewRunner(buildRootDir, files, flags["showlog"])

		cr.Open(outFile)
		outStream = cr.WriteStream()
	}
	if action == kSilent {
		outStream = io.Discard
	}

	p := program.NewProgram(env, es)
	fname := filepath.Base(filePath)
	fname = fname[:len(fname)-3]

	p.ParseRoot(fname)
	if !es.Clean() {
		return es
	}

	// output phase
	w := textwriter.NewWriter(outStream)
	cw := cwriter.NewCWriter(w)
	cw.WriteProgram(p)

	if !(action == kPrint || action == kSilent) {
		defs := map[string]string{
			"EYOT_RUNTIME_MAX_ARGS": fmt.Sprintf("%v", p.Functions.MaxArgCount()),
		}

		ffiFiles := []string{}
		for _, mod := range p.Modules {
			if mod.Ffid != nil {
				ffiFiles = append(ffiFiles, mod.Ffid.Src)
			}
		}

		log, err := cr.Close(action != kPrepare, p.GpuRequired, defs, ffiFiles, p.FfiFlags())
		if action == kPrepare {
			if err != nil {
				es.LogInternalError(fmt.Errorf("Code preparation error: %v", err))
				return es
			}

			// this is the folder path
			fmt.Println(buildRootDir)
		} else {
			if err != nil {
				fmt.Println(log)
				es.LogInternalError(fmt.Errorf("CC error: %v", err))
				return es
			}
		}
	}

	if action == kRun {
		var stdOutStream io.Writer = os.Stdout

		var ocmd *exec.Cmd = nil
		if useOclGrind {
			ocmd = exec.Command("oclgrind", outFile)
		} else {
			ocmd = exec.Command(outFile)
		}

		ocmd.Stdin = os.Stdin
		ocmd.Stdout = stdOutStream
		ocmd.Stderr = os.Stderr
		err := ocmd.Run()

		if err != nil {
			// not obvious this is an error once we've added exit codes
			es.LogInternalError(fmt.Errorf("Error running: %v", err))
			return es
		}
	}

	return nil
}

func main() {
	es := errMain()
	if es == nil {
		os.Exit(0)
	} else if es.Clean() {
		os.Exit(0)
	} else {
		if es.InternalError() != nil {
			fmt.Println("internal error: ", es.InternalError())
		} else {
			es.LogErrors(os.Stdout)
		}
		os.Exit(1)
	}
}
