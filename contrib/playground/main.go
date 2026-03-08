package main

import (
	"os"
	"os/exec"
	"io"
	"fmt"
	"time"
	"bytes"
	"strings"
	"strconv"
	"path/filepath"
	"text/template"
	"encoding/json"
	"net/http"
	"embed"
)

// arbitrary, but 128k seems reasonable max for the playground
const KMaxRequestLength int = 1024 * 128

//go:embed index.html
var IndexHtmlTemplate string

//go:embed examples
var EmbeddedExamples embed.FS

func Log(msg string) {
	fmt.Printf("[%v] %v\n", time.Now().Format("2006-01-02 15:04:05"), msg)
}

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

	if len(j.Request.Source) > KMaxRequestLength {
		e.Encode(&JobResponse {
			CompileSuccess: false,
			CompileLog: fmt.Sprintf("Request too long (max = %v bytes)", KMaxRequestLength),
			Log: "",
		})
		j.Done <- true
		return nil
	}
	
	if cr, ok := r.CachedResponses[j.Request.Source]; ok {
		Log(fmt.Sprintf("Cache hit for job %v (%v ms)", j.Id, j.Lifetime()))
		jr = cr
	} else {
		Log(fmt.Sprintf("Start job %v (%v ms)", j.Id, j.Lifetime()))

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
			Log("No stdout for compile")
			return fmt.Errorf("Unable to create stdout: %v", err)
		}
		if err := compileCmd.Start(); err != nil {
			Log("No start for compile")
			return fmt.Errorf("Failed to start: %v", err)
		}
		compileLog, _ := io.ReadAll(compileStdOut)
		compileCmd.Wait()
		if compileCmd.ProcessState == nil {
			Log("No process state for compile")
			return fmt.Errorf("Process failed to set state")
		}
		if compileCmd.ProcessState.ExitCode() != 0 {
			Log("Failed to compile")
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
			"--quiet",
			// we run in the /var/lib folder passed by systemd
			"--writable-var",
			"oclgrind",
			"./out.exe",
		)
		runStdOut, err := runCmd.StdoutPipe()
		if err != nil {
			Log("No stdout for run")
			return fmt.Errorf("Unable to create stdout for run: %v", err)
		}
		runStdErr, err := runCmd.StderrPipe()
		if err != nil {
			Log("No stderr for run")
			return fmt.Errorf("Unable to create stderr for run: %v", err)
		}
		if err := runCmd.Start(); err != nil {
			Log("No start for run")
			return fmt.Errorf("Failed to start for run: %v", err)
		}

		returnOut := make(chan string)
		returnErr := make(chan string)

		go func() {
			lg, _ := io.ReadAll(runStdOut)
			returnOut <- string(lg)
		}()
		go func() {
			lg, _ := io.ReadAll(runStdErr)
			returnErr <- string(lg)
		}()

		runCmd.Wait()
		runLog := <- returnOut
		runErr := <- returnErr

		jr = JobResponse {
			CompileSuccess: true,
			CompileLog: string(compileLog),
			Log: string(runLog + runErr),
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

func EscapeExample(s string) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

func ReadExamples() (Examples, error) {
	root := "examples"

	egs, err := EmbeddedExamples.ReadDir(root)
	if err != nil {
		return Examples {}, err
	}

	items := []Example {}

	for _, eg := range egs {
		if !eg.IsDir() {
			continue
		}

		main := filepath.Join(root, eg.Name(), "main.ey")
		mainBytes, err := EmbeddedExamples.ReadFile(main)
		if err != nil {
			return Examples {}, err
		}

		desc := filepath.Join(root, eg.Name(), "description.txt")
		descBytes, err := EmbeddedExamples.ReadFile(desc)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return Examples {}, err
		}

		eg := Example {
			Id: eg.Name(),
			Description: string(descBytes),
			Contents: EscapeExample(string(mainBytes)),
		}
		items = append(items, eg)
	}

	return Examples { Items: items }, nil
}

type Server struct {
	JobChannel chan *Job
	IndexHtml string
}

func NewServer() (*Server, error) {
	examples, err := ReadExamples()
	if err != nil {
		return nil, err
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
			Log(fmt.Sprintf("runner error: %v", err))
		}
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	Log(fmt.Sprintf("Serve %v", req.URL))

	if req.URL.Path == "/api/run" {
		d := json.NewDecoder(req.Body)

		var p Program
		if err := d.Decode(&p); err != nil {
			Log(fmt.Sprintf("Failed to decode job: %v", err))
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

		Log(fmt.Sprintf("Finished job %v (%v ms)", job.Id, job.Lifetime()))
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
	Log(fmt.Sprintf("Listen on port: %v", port))
	Log(fmt.Sprintf("Job folder: %v", workingFolder))
	return http.ListenAndServe(fmt.Sprintf(":%v", port), s)
}

func main() {
	if err := errMain(); err != nil {
		fmt.Println("fatal: ", err)
		os.Exit(1)
	}
}
