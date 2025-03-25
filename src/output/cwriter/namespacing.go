package cwriter

import (
	"eyot/ast"
	"fmt"
	"strings"
)

func escapeModuleCpt(cpt string) string {
	// there can be an issue with dashes in module names
	return strings.ReplaceAll(cpt, "-", "_")
}

func namespaceStructId(sid ast.StructId) string {
	structNamespace := ""
	for i, cpt := range sid.Module {
		if i > 0 {
			structNamespace += "_"
		}
		structNamespace += escapeModuleCpt(cpt)
	}
	return structNamespace + "__" + sid.Name
}

// core function namespacing, generally called via a number of other options
func namespaceFunctionCore(fid ast.FunctionId) string {
	if fid.Struct.Blank() {
		if fid.Module.IsBuiltin() {
			return fid.Name
		} else {
			return fid.Module.Namespace() + "___unbound___" + fid.Name
		}
	} else {
		// the fid namespace is largely irrelevant as the struct carries the relevant namespace
		// the fid namespace also seems to be wrong here, so another good reason to ignore
		return namespaceStructId(fid.Struct) + "___bound___" + fid.Name
	}
}

// a wrapper for namespaceFunction that
func namespaceFunctionId(fid ast.FunctionId) string {
	return "ey_function_" + namespaceFunctionCore(fid)
}

// function caller wrappers
func namespaceFunctionCallerId(fid ast.FunctionId) string {
	return "ey_function_caller_" + namespaceFunctionCore(fid)
}

// the core function caller
func namespaceCentralFunctionCaller() string {
	return "ey_functioncaller"
}

// namespace a function id to an enum key for the full function
func namespaceFunctionEnumId(fid ast.FunctionId) string {
	return "k_ey_function_" + namespaceFunctionCore(fid)
}

func namespaceFunctionCount() string {
	// no underscore to guarantee no collisions
	return "k_ey_max_arg_count"
}

// Eyot-side struct name to generated C struct name
func namespaceStruct(sid ast.StructId) string {
	if sid.Module.IsBuiltin() {
		return sid.Name
	} else {
		return "ey_struct_" + namespaceStructId(sid)
	}
}

func namespaceClosureArgSize() string {
	return "ey_generated_closure_arg_size"
}

/*
These are not generated yet, but they will be eventually

	func namespaceClosureArgOffset() string {
		return "ey_generated_closure_arg_offset"
	}

	func namespaceClosureSize() string {
		return "ey_generated_closure_size"
	}
*/
func namespaceEnumFunctionListNew() string {
	return "EyRuntimeFunctionList"
}

func namespaceStringPoolStringUtf32(i int) string {
	return fmt.Sprintf("ey_string_pool_raw_u32_%v", i)
}

func namespaceStringPoolName() string {
	return "ey_string_pool_raw"
}

func namespaceStringPoolGet() string {
	return "ey_runtime_string_get"
}

func namespaceExecutionContext() string {
	return "ey_execution_context"
}

func namespaceUseStringLiteral() string {
	return "ey_runtime_string_use_literal"
}

func namespaceWorkerFunction() string {
	return "EyWorkerFunction"
}
