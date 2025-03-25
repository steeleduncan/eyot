package program

import (
	"eyot/ast"
	"os"
	"path/filepath"
)

// build time setter (-ldflags "-X eyot.program.EyotRoot=/usr/share/blah")
var EyotRoot string

type Environment struct {
	// root path of the std library
	Roots []string
}

// default env creator, pulls from environment or build settings
func CreateEnvironment(localPath string) *Environment {
	root := os.Getenv("EyotRoot")
	if root == "" {
		root = EyotRoot
	}
	if root == "" {
		panic("EyotRoot is not set")
	}

	return &Environment{
		Roots: []string{localPath, root},
	}
}

func (e *Environment) RuntimeRoot() string {
	// TODO test properly here
	return filepath.Join(e.Roots[1], "runtime")
}

// Find a path to a module
func (e *Environment) FindModule(cpts ast.ModuleId) string {
	for _, root := range e.Roots {
		path := filepath.Join(root, filepath.Join(cpts...)) + ".ey"
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}
