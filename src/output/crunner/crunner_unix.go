//go:build linux || darwin

package crunner

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type CRunnerUnix struct {
	outBuf    *bytes.Buffer
	cmd       *exec.Cmd
	echan     chan error
	outStream io.WriteCloser
	ip        string
	files     []string
	outFile   string
	showLog   bool
}

var _ CRunner = &CRunnerUnix{}

func NewRunner(includePath string, runtimeFiles []string, showOutput bool) CRunner {
	return &CRunnerUnix{
		ip:      includePath,
		files:   runtimeFiles,
		showLog: showOutput,
	}
}

func (cr *CRunnerUnix) Open(outFile string) error {
	cr.outBuf = bytes.NewBuffer([]byte{})
	cr.outFile = outFile

	var err error
	cr.outStream, err = os.Create(filepath.Join(cr.ip, "eyot-main.c"))
	if err != nil {
		return fmt.Errorf("failed to create eyot main: %v", err)
	}

	cr.files = append(cr.files, "eyot-main.c")

	return nil
}

func (cr *CRunnerUnix) WriteStream() io.Writer {
	return cr.outStream
}

func DebugMode() bool {
	return os.Getenv("EyotDebug") == "y"
}

func (cr *CRunnerUnix) Close(compile bool, withOpenCl bool, defs map[string]string, supportFiles []string, flags map[string]bool) (string, error) {
	cr.outStream.Close()

	for sfi, sf := range supportFiles {
		fnam := fmt.Sprintf("eyot-ffi-%v.c", sfi)
		path := filepath.Join(cr.ip, fnam)
		err := os.WriteFile(path, []byte(sf), 0777)
		if err != nil {
			return "", fmt.Errorf("Failed to write output file %v: %v", path, err)
		}
		cr.files = append(cr.files, fnam)
	}

	if compile {
		cr.cmd = cr.getCompileCommand(withOpenCl, defs, flags)
		cr.cmd.Stdout = cr.outBuf
		cr.cmd.Stderr = cr.outBuf

		err := cr.cmd.Run()
		if err != nil {
			return cr.outBuf.String(), fmt.Errorf("CC error: %v", err)
		}
		if strings.Contains(cr.outBuf.String(), "warning") || cr.showLog {
			indent := "  >  "
			fmt.Println("Build output from C compiler:")
			fmt.Println(indent + strings.ReplaceAll(cr.outBuf.String(), "\n", "\n"+indent))
			fmt.Println()
			fmt.Println()
		}
	}

	return "", nil
}

// check for clang
// NB on macOS 'gcc' actually resolves to clang, but is not compatible with gcc
func cmdIsClang(cc string) bool {
	outBuf := bytes.NewBuffer([]byte{})
	cmd := exec.Command(cc, "--version")
	cmd.Stdout = outBuf
	if err := cmd.Run(); err != nil {
		// don't realy
		return false
	}

	return strings.Contains(strings.ToLower(outBuf.String()), "clang")
}

func (cr *CRunnerUnix) getCompileCommand(withOpenCl bool, defs map[string]string, flags map[string]bool) *exec.Cmd {
	// should work for gcc, tcc, clang right now
	cc := "gcc"
	if altCC := os.Getenv("CC"); altCC != "" {
		cc = altCC
	}

	args := []string{}
	args = append(args, "-g3")
	if DebugMode() {
		args = append(args, "-fsanitize=address,undefined")
	}

	if !cr.showLog {
		if cmdIsClang(cc) {
			args = append(args, "-Wno-everything")
		} else {
			args = append(args, "-w")
		}
	}


	args = append(args, "-std=c99")

	args = append(args, "-o")
	args = append(args, cr.outFile)

	if withOpenCl {
		args = append(args, "-DEYOT_OPENCL_INCLUDED")
	}

	if cr.showLog {
		args = append(args, "-DEYOT_SHOW_LOG")
	}

	for key, value := range defs {
		args = append(args, "-D"+key+"="+value)
	}

	for _, f := range cr.files {
		args = append(args, filepath.Join(cr.ip, f))
	}

	if withOpenCl {
		args = append(args, openCLArgs()...)
	}

	for flag, _ := range flags {
		args = append(args, flag)
	}

	return exec.Command(cc, args...)
}
