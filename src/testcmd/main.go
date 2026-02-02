package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func sortLineEndings(contents string) string {
	contents = strings.TrimSpace(contents)
	contents = strings.Replace(contents, "\r", "", -1)

	return contents
}

func usage() error {
	fmt.Println(`eyot/testcmd <binary path> <root folder>`)
	return nil
}

func runTest(sourcePath, outputPath string, isOut, isErr bool) (bool, bool, error) {
	useOclGrind := os.Getenv("EyotTestOclGrind") == "y"
	binaryPath := os.Args[1]

	testOutputBlob, err := os.ReadFile(outputPath)
	if err != nil {
		return false, false, fmt.Errorf("Failed to read output at '%v': %v", outputPath, err)
	}
	referenceOutput := sortLineEndings(string(testOutputBlob))

	buf := bytes.NewBuffer([]byte{})
	var cmd *exec.Cmd
	if useOclGrind {
		cmd = exec.Command("oclgrind", binaryPath, "run", sourcePath)
	} else {
		cmd = exec.Command(binaryPath, "run", sourcePath)
	}
	cmd.Stdout = buf
	cmd.Stderr = buf
	cmd.Env = os.Environ()

	// the sanitiser seems incompatible with some dlopen parameter used by open cl
	if strings.Contains(sourcePath, "/gpu/") && os.Getenv("EyotDebug") == "y" {
		cmd.Env = append(cmd.Env, "EyotDebug=n")
		fmt.Println("  Disable sanitiser")
	}

	testRunError := cmd.Run()

	actualOutput := sortLineEndings(buf.String())
	if strings.Contains(actualOutput, "ey-test-reserved-pass") {
		fmt.Println("  Forced pass")
		return true, false, nil
	}

	if isOut {
		if testRunError != nil {
			fmt.Printf("  Execution of '%v' failed with\n    %v\n", sourcePath, testRunError)
			return false, false, nil
		}
		if actualOutput != referenceOutput {
			fmt.Printf("  Bad output when running '%v'\n    Got '%v'\n", sourcePath, actualOutput)

			// This appears to be a transient issue with oclgrind
			// It shows up sometimes in GHA on the ubuntu 24.04 machine
			// I don't believe it is an actual issue with Eyot, so a retry seems reasonable
			if strings.Contains(actualOutput, "[Oclgrind] Failed to get path to library") {
				return false, true, nil
			}

			return false, false, nil
		}
	} else if isErr {
		actualOutput = strings.ToLower(actualOutput)
		referenceOutput = sortLineEndings(strings.ToLower(referenceOutput))

		if testRunError == nil {
			fmt.Printf("  No error when running '%v'\n", sourcePath)
			return false, false, nil
		}

		if !strings.Contains(actualOutput, referenceOutput) {
			fmt.Printf("  Bad output when running '%v'\n    Got '%v'\n", sourcePath, actualOutput)
			return false, false, nil
		}
	}

	return true, false, nil
}

func runTestWithRetries(sourcePath, outputPath string, isOut, isErr bool, retries int) (bool, error) {
	for i := 0; i < retries; i += 1 {
		fmt.Printf("%v (%v / %v)\n", sourcePath, i + 1, retries)

		pass, retry, err := runTest(sourcePath, outputPath, isOut, isErr)
		if err != nil {
			return false, err
		}

		if pass {
			return true, nil
		}

		if !retry {
			break
		}
	}

	return false, nil
}

func errMain() error {
	if len(os.Args) != 3 {
		return usage()
	}

	rootFolder := os.Args[2]

	failures := []string{}

	err := filepath.Walk(rootFolder, func(outputPath string, info os.FileInfo, err error) error {
		nm := filepath.Base(outputPath)

		if strings.HasPrefix(nm, ".") {
			fmt.Println("skip")
			return nil
		}

		isOut := strings.HasSuffix(nm, ".out.txt")
		isErr := strings.HasSuffix(nm, ".err.txt")

		if isOut || isErr {
			sourcePath := outputPath[:len(outputPath)-7] + "ey"

			pass, err := runTestWithRetries(sourcePath, outputPath, isOut, isErr, 3)
			if err != nil {
				return err
			}

			if !pass {
				failures = append(failures, outputPath)
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	if len(failures) > 0 {
		buf := bytes.NewBuffer([]byte{})
		fmt.Fprintf(buf, "%v failures: ", len(failures))
		for _, fail := range failures {
			fmt.Fprintln(buf)
			fmt.Fprintf(buf, "- %v", fail)
		}
		return fmt.Errorf(buf.String())
	}

	return nil
}

func main() {
	err := errMain()
	if err != nil {
		fmt.Println("fatal: ", err)
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}
