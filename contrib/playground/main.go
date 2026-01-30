package main

import (
	"os"
	"os/exec"
	"io"
	"fmt"
	"path/filepath"
	"encoding/json"
	"net/http"
	_ "embed"
)

import _ "embed"

//go:embed index.html
var IndexHtml string

type Program struct {
	Version int
	Source string
}

type Result struct {
	ErrorLog string
	Output string
}

type JobRunner struct {
	Path string
}

type Job struct {
	Request Program

	// write here
	w http.ResponseWriter

	// any value here when finished
	Done chan bool
}


type JobResponse struct {
	CompileSuccess bool
	CompileLog string

	Log string
}

func (r *JobRunner) Run(j *Job) error {
	fmt.Println("Run ", j.Request.Source)
	os.RemoveAll(r.Path)
	os.MkdirAll(r.Path, 0777)

	j.w.Header().Set("Content-Type", "application/json")
	j.w.WriteHeader(http.StatusOK)

	e := json.NewEncoder(j.w)

	// write start file
	source := filepath.Join(r.Path, "main.ey")
	os.WriteFile(source, []byte(j.Request.Source), 0777)

	// compile
	os.Chdir(r.Path)
	compileCmd := exec.Command("eyot", "build", source)
	compileStdOut, err := compileCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("Unable to create stdout: %v", err)
	}
	if err := compileCmd.Start(); err != nil {
		return fmt.Errorf("Failed to start: %v", err)
	}
	compileLog, _ := io.ReadAll(compileStdOut)
	compileCmd.Wait()
	if compileCmd.ProcessState == nil {
		return fmt.Errorf("Process failed to set state")
	}
	if compileCmd.ProcessState.ExitCode() != 0 {
		e.Encode(&JobResponse {
			CompileSuccess: false,
			CompileLog: string(compileLog),
		})
		fmt.Println("compile failed: ", compileLog)
		j.Done <- true
		return nil
	}

	// run
	runCmd := exec.Command(
		"timeout", "10s",
		"firejail",
		//		"--cpu.quota=20",
		"--deterministic-shutdown",
		"--net=none",
		"--private=" + r.Path,
		"--nodbus",
		"--noprofile",
		"--nosound",
		"--noinput",
		"oclgrind",
		"./out.exe",
	)
	runStdOut, err := runCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("Unable to create stdout for run: %v", err)
	}
	if err := runCmd.Start(); err != nil {
		return fmt.Errorf("Failed to start for run: %v", err)
	}
	runLog, _ := io.ReadAll(runStdOut)
	runCmd.Wait()

	e.Encode(&JobResponse {
		CompileSuccess: true,
		CompileLog: string(compileLog),
		Log: string(runLog),
	})
	j.Done <- true
	return nil
}

type Server struct {
	JobChannel chan *Job
}

func NewServer() *Server {
	s := &Server {
		JobChannel: make(chan *Job),
	}

	go s.RunJobs()

	return s
}

func (s *Server) RunJobs() {
	runner := &JobRunner {
		Path: "/tmp/eyot-playground-job-runner",
	}
	
	for {
		job := <- s.JobChannel
		err := runner.Run(job)
		if err != nil {
			fmt.Println("runner error: ", err)
		}
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/run" {
		d := json.NewDecoder(req.Body)

		var p Program
		if err := d.Decode(&p); err != nil {
			fmt.Println("Failed to decode job: ", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		job := &Job {
			Request: p,
			w: w,
			Done: make(chan bool),
		}

		s.JobChannel <- job
		_ = <- job.Done
	} else {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, IndexHtml)
	}
}

func errMain() error {
	s := NewServer()
	return http.ListenAndServe(":12321", s)
}

func main() {
	if err := errMain(); err != nil {
		fmt.Println("fatal: ", err)
		os.Exit(1)
	}
}
