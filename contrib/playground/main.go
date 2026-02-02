package main

import (
	"os"
	"os/exec"
	"io"
	"fmt"
	"time"
	"bytes"
	"strconv"
	"path/filepath"
	"text/template"
	"encoding/json"
	"net/http"
	_ "embed"
)

import _ "embed"

//go:embed index.html
var IndexHtmlTemplate string

type Program struct {
	Version int
	Source string
}

type JobResponse struct {
	CompileSuccess bool
	CompileLog string
	Log string
}

type JobRunner struct {
	Path string
	RollingId int
	CachedResponses map[string]JobResponse
}

type Job struct {
	Request Program

	// write here
	w http.ResponseWriter

	// any value here when finished
	Done chan bool

	StartTime int64

	Id int
}

func (j *Job) Lifetime() int64 {
	return (time.Now().UnixNano() - j.StartTime) / 1000000
}

func (r *JobRunner) Run(j *Job) error {
	j.Id = r.RollingId
	r.RollingId += 1
	j.w.Header().Set("Content-Type", "application/json")
	j.w.WriteHeader(http.StatusOK)

	var jr JobResponse
	e := json.NewEncoder(j.w)
	
	if cr, ok := r.CachedResponses[j.Request.Source]; ok {
		fmt.Printf("Cache hit for job %v (%v ms)\n", j.Id, j.Lifetime())
		jr = cr
	} else {
		fmt.Printf("Start job %v (%v ms)\n", j.Id, j.Lifetime())

		os.RemoveAll(r.Path)
		os.MkdirAll(r.Path, 0777)


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
			j.Done <- true
			return nil
		}

		// run
		runCmd := exec.Command(
			"timeout", "10s",
			"firejail",
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

		jr = JobResponse {
			CompileSuccess: true,
			CompileLog: string(compileLog),
			Log: string(runLog),
		}

		r.CachedResponses[j.Request.Source] = jr
	}

	e.Encode(&jr)
	j.Done <- true
	return nil
}

type Example struct {
	Id string
	Description string
	Contents string
}
type Examples struct {
	Items []Example
}

type Server struct {
	JobChannel chan *Job
	IndexHtml string
}

func NewServer() (*Server, error) {
	examples := Examples {
		Items: []Example {
			// move these to YAMLs or something similar?
			Example {
				Id: "hello",
				Description: "Minimal hello world",
				Contents: `cpu fn main() {\n    print_ln("Hello, World!")\n}`,
			},
			Example {
				Id: "gpusquare",
				Description: "Square a vector of numbers on the GPU",
				Contents: `fn square(value i64) i64 {\n   return value * value\n}\n\ncpu fn main() {\n    let w = gpu square\n    send(w, [i64]{ 1, 2, 3, 4 })\n    for v: drain(w) {\n        print_ln("- ", v)\n    }\n}`,
			},
			Example {
				Id: "partial",
				Description: "Partial function application",
				Contents: `fn multiply(lhs, rhs i64) i64 {\n   return lhs * rhs \n}\n cpu fn main() {\n    let dbl = partial multiply(_, 2)\n    print_ln(dbl(3))\n}`,
			},
		},
	}

	tmpl, err := template.New("name").Parse(IndexHtmlTemplate)
	if err != nil {
		return nil, fmt.Errorf("Error parsing template")
	}
	buf := bytes.NewBuffer([]byte {})
	if err := tmpl.Execute(buf, examples); err != nil {
		return nil, fmt.Errorf("Error executing template: %v")
	}

	s := &Server {
		JobChannel: make(chan *Job),
		IndexHtml: buf.String(),
	}

	return s, nil
}

func (s *Server) RunJobsInBackground(path string) {
	runner := &JobRunner {
		Path: path,
		CachedResponses: map[string]JobResponse {},
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
	if req.URL.Path == "/api/run" {
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
			StartTime: time.Now().UnixNano(),
		}

		s.JobChannel <- job
		_ = <- job.Done

		fmt.Printf("Finished job %v (%v ms)\n", job.Id, job.Lifetime())
	} else {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, s.IndexHtml)
	}
}

func usage() error {
	fmt.Println("playground: <port> <working folder>")
	return nil
}

func errMain() error {
	if len(os.Args) != 3 {
		return usage()
	}

	portArg := os.Args[1]
	workingFolder := os.Args[2]

	port, err := strconv.Atoi(portArg)
	if err != nil {
		return fmt.Errorf("Unable to understand port argument: %v", portArg)
	}
	if port == 0 {
		return usage()
	}


	s, err := NewServer()
	if err != nil {
		return err
	}
	go s.RunJobsInBackground(workingFolder)
	fmt.Println("Listen on port: ", port)
	fmt.Println("Job folder: ", workingFolder)
	return http.ListenAndServe(fmt.Sprintf(":%v", port), s)
}

func main() {
	if err := errMain(); err != nil {
		fmt.Println("fatal: ", err)
		os.Exit(1)
	}
}
