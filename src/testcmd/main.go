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

func errMain() error {
	if len(os.Args) != 3 {
		return usage()
	}

	binaryPath := os.Args[1]
	rootFolder := os.Args[2]

	useOclGrind := os.Getenv("EyotTestOclGrind") == "y"

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
			fmt.Println(sourcePath)
			testOutputBlob, err := os.ReadFile(outputPath)
			if err != nil {
				return fmt.Errorf("Failed to read output at '%v': %v", outputPath, err)
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
				return nil
			}

			pass := true

			if isOut {
				if testRunError != nil {
					fmt.Printf("  Execution of '%v' failed with\n    %v\n", sourcePath, testRunError)
					pass = false
				}
				if actualOutput != referenceOutput {
					fmt.Printf("  Bad output when running '%v'\n    Got '%v'\n", sourcePath, actualOutput)
					pass = false
				}
			} else if isErr {
				actualOutput = strings.ToLower(actualOutput)
				referenceOutput = sortLineEndings(strings.ToLower(referenceOutput))

				if testRunError == nil {
					fmt.Printf("  No error when running '%v'\n", sourcePath)
					pass = false
				}

				if !strings.Contains(actualOutput, referenceOutput) {
					fmt.Printf("  Bad output when running '%v'\n    Got '%v'\n", sourcePath, actualOutput)
					pass = false
				}
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
