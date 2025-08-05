package program

import (
	"encoding/json"
	"eyot/ast"
	"fmt"
	"os"
	"strings"
)

type externalFfiFunctionDefinition struct {
	Name      string
	Arguments []string
	Return    string
}

type externalFfiDefinitions struct {
	Functions   []externalFfiFunctionDefinition
	LinkerFlags []string
}

func convertFfiType(tyname string) (ast.Type, error) {
	tyname = strings.TrimSpace(tyname)
	if len(tyname) >= 3 {
		if tyname[0] == '[' && tyname[len(tyname)-1] == ']' {
			tyname = tyname[1:(len(tyname) - 1)]
			ty, err := convertFfiType(tyname)
			if err != nil {
				return ast.Type{}, fmt.Errorf("Parsing error in vector: %v", err)
			}
			return ast.MakePointer(ast.Type{
				Selector: ast.KTypeVector,
				Types:    []ast.Type{ty},
			}), nil
		}
	}

	switch tyname {
	case "EyInteger":
		return ast.Type{Selector: ast.KTypeInteger}, nil

	case "EyBoolean":
		return ast.Type{Selector: ast.KTypeBoolean}, nil

	case "EyString":
		return ast.Type{Selector: ast.KTypeString}, nil

	case "EyFloat32":
		return ast.Type{ Selector: ast.KTypeFloat, Width: 32 }, nil

	case "EyFloat64":
		return ast.Type{ Selector: ast.KTypeFloat, Width: 64 }, nil

	case "":
		return ast.Type{Selector: ast.KTypeVoid}, nil

	default:
		return ast.Type{}, fmt.Errorf("Do not recognise type in ffi declaration: '%v'", tyname)
	}
}

func (ef *externalFfiDefinitions) Convert() (*ast.FfiDefinitions, error) {
	ffid := &ast.FfiDefinitions{
		Functions:   []ast.CFunction{},
		LinkerFlags: []string{},
	}

	if ef.LinkerFlags != nil {
		ffid.LinkerFlags = ef.LinkerFlags
	}

	for _, fn := range ef.Functions {
		newFn := ast.CFunction{
			Name:          fn.Name,
			ArgumentTypes: []ast.Type{},
		}

		var err error
		newFn.ReturnType, err = convertFfiType(fn.Return)
		if err != nil {
			return nil, err
		}

		for _, ffiType := range fn.Arguments {
			ty, err := convertFfiType(ffiType)
			if err != nil {
				return nil, err
			}

			newFn.ArgumentTypes = append(newFn.ArgumentTypes, ty)
		}

		ffid.Functions = append(ffid.Functions, newFn)
	}

	return ffid, nil
}

func FfiAt(path string) (*ast.FfiDefinitions, error) {
	blob, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}

	var effid externalFfiDefinitions
	err = json.Unmarshal(blob, &effid)
	if err != nil {
		return nil, fmt.Errorf("Failed to read json at %v: %v", path, err)
	}

	converted, err := effid.Convert()
	if err != nil {
		return nil, fmt.Errorf("Failed to unpack ffi json at %v: %v", path, err)
	}

	cPath := path[:len(path)-4] + "c"
	cBlob, err := os.ReadFile(cPath)
	if err != nil {
		return nil, fmt.Errorf("No c file found for ffi json: %v", cPath)
	}

	converted.Src = string(cBlob)
	return converted, nil
}
