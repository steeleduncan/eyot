package crunner

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type CRunnerWindows struct {
	buildRoot, exePath, tempPath, outPath, runtimePath string
	outStream                                          io.WriteCloser
	msvcIncludes, msvcLibs                             []string
	msvcPath                                           string
}

var _ CRunner = &CRunnerWindows{}

func NewRunner(buildPath string) CRunner {
	cr := &CRunnerWindows{
		msvcIncludes: []string{},
		msvcLibs:     []string{},
		buildRoot:    buildPath,
		runtimePath:  filepath.Join(buildPath, "eyot-runtime.c"), // ideally this isn't hard coded
		tempPath:     filepath.Join(buildPath, fmt.Sprintf("temp-file-%v.c", os.Getpid())),
		exePath:      filepath.Join(buildPath, fmt.Sprintf("temp-file-%v.exe", os.Getpid())),
	}
	cr.setupMsvcPaths()
	return cr
}

func (cr *CRunnerWindows) Open(path string) error {
	cr.outPath = path
	fh, err := os.Create(cr.tempPath)
	if err != nil {
		return fmt.Errorf("CRunnerWindows unable to create temporary C file: %v", err)
	}
	cr.outStream = fh

	return nil
}

func (cr *CRunnerWindows) WriteStream() io.Writer {
	return cr.outStream
}

func (cr *CRunnerWindows) Close() (string, error) {
	cr.outStream.Close()
	//defer os.Remove(cr.tempPath)

	outBuf := bytes.NewBuffer([]byte{})

	os.Chdir(cr.buildRoot)

	// source file
	args := []string{cr.tempPath, cr.runtimePath}

	// compile options
	for _, path := range cr.msvcIncludes {
		args = append(args, "/I", path)
	}

	// get rid of this file
	//args = append(args, "/Fo:" + filepath.Join(os.TempDir(), "junk.obj"))

	// swap to linker
	args = append(args, "/link")

	// linker options
	for _, path := range cr.msvcLibs {
		args = append(args, "/LIBPATH", path)
	}
	args = append(args, "/out:"+cr.exePath)

	// TODO linker part needs something to suppress the .obj file (or send it somewhere innocuous)

	cmd := exec.Command(filepath.Join(cr.msvcPath, "cl.exe"), args...)

	cmd.Stdout = outBuf
	cmd.Stderr = outBuf

	err := cmd.Run()
	if err != nil {
		return outBuf.String(), fmt.Errorf("CC error: %v", err)
	}

	os.Rename(cr.exePath, cr.outPath)

	return "", nil
}

func (cr *CRunnerWindows) getCompileCommandTcc() *exec.Cmd {
	return exec.Command("tcc", "-o", cr.outPath, cr.tempPath)
}

func (cr *CRunnerWindows) setupMsvcPaths() {
	// msvcVersion = "14.34.31933"
	msvcVersion := "14.35.32215"

	buildToolsRoot := `C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Tools\MSVC\` + msvcVersion
	windowsKitRoot := `C:\Program Files (x86)\Windows Kits\10`
	windowsKitVersion := "10.0.22621.0"
	targetArch := "x64"
	hostArch := "Hostx64"

	cr.msvcPath = filepath.Join(buildToolsRoot, "bin", hostArch, targetArch)
	if os.Getenv("INCLUDE") == "" {
		cr.msvcIncludes = append(cr.msvcIncludes,
			filepath.Join(windowsKitRoot, "Include", windowsKitVersion, "ucrt"),
			filepath.Join(buildToolsRoot, "include"),
		)
	}
	if os.Getenv("LIB") == "" {
		cr.msvcLibs = append(cr.msvcLibs,
			filepath.Join(windowsKitRoot, "Lib", windowsKitVersion, "ucrt", targetArch),
			//filepath.Join(windowsKitRoot, "Lib", windowsKitVersion, "um", targetArch),
			filepath.Join(buildToolsRoot, "Lib", targetArch),
		)
	}
}
