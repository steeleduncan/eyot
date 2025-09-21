package ast

import (
	"eyot/errors"
	"fmt"
)

// A resolved struct, ie one that has been created
type RequiredStruct struct {
	// When true this definition is internally generated to hold a tuple
	GeneratedForTuple bool

	// unique A lookup id so we don't include the same thing many times
	TypeId string

	// name for this struct
	Id StructId

	Definition StructDefinition
}

// A type of vector used in the program
// A number of functions must be generated to support this
type RequiredVector struct {
	// unique A lookup id so we don't include the same thing many times
	ElementType Type
}

type CheckPass int

const (
	// Set cached types everywhere
	KPassSetTypes CheckPass = iota

	// Mutate the tree as needed
	KPassMutate

	// Check types (including from mutated code)
	KPassCheckTypes
)

/*
   A Check context is the overall record of typechecking a module
 */
type CheckContext struct {
	Pass                  CheckPass
	Errors                *errors.Errors
	Structs               []*RequiredStruct
	Vectors               map[string]Type
	insertStatements      [][]Statement
	insertElements        []TopLevelElement
	returnTypes           []Type
	tempCount             int
	shouldRemoveStatement bool
	maximumClosureSize    int

	// map from function key to a function (only gpu functions are required here, so only gpu functions are kept)
	Functions *FunctionGroup

	// when positive we are in a cpu method
	inCpuMethodCount, inGpuMethodCount int

	// When true the GPU was required for at least something in the code
	gpuRequired bool

	// A map from string contents to the pool id
	Strings map[string]int

	currentModule *Module
}

func NewCheckContext(es *errors.Errors, sp map[string]int) *CheckContext {
	return &CheckContext{
		Errors:                es,
		Structs:               []*RequiredStruct{},
		Vectors:               map[string]Type {},
		returnTypes:           []Type{},
		insertStatements:      [][]Statement{},
		insertElements:        []TopLevelElement{},
		shouldRemoveStatement: false,
		inCpuMethodCount:      0,
		inGpuMethodCount:      0,
		maximumClosureSize:    0,
		tempCount:             0,
		Strings:               sp,
		gpuRequired:           false,
		Functions:             NewFunctionGroup(),
	}
}

func (ctx *CheckContext) GetStringId(s string) int {
	id, found := ctx.Strings[s]
	if found {
		return id
	}

	id = len(ctx.Strings)
	ctx.Strings[s] = id
	return id
}

func (ctx *CheckContext) RequireClosureSize(size int) {
	if size > ctx.maximumClosureSize {
		ctx.maximumClosureSize = size
	}
}

func (ctx *CheckContext) AssertType(e Expression, ty TypeSelector) {
	if e.Type().Selector != ty {
		ctx.Errors.Errorf("Mismatched types: expecting %v, got %v", RoughTypeName(ty), e.Type())
	}
}

func (ctx *CheckContext) MaximumClosureSize() int {
	return ctx.maximumClosureSize
}

func (ctx *CheckContext) CurrentModule() *Module {
	return ctx.currentModule
}

func (cc *CheckContext) GpuRequired() bool {
	return cc.gpuRequired
}

func (cc *CheckContext) SetGpuRequired() {
	cc.gpuRequired = true
}

func (cc *CheckContext) GetUniqueId() int {
	cc.tempCount += 1
	return cc.tempCount
}

func (cc *CheckContext) GetTemporaryName() string {
	return fmt.Sprintf("ey_temp_%v", cc.GetUniqueId())
}

func (cc *CheckContext) validateCpuGpuCount() {
	if cc.inCpuMethodCount < 0 {
		panic("CheckContext: negative cpu method count")
	}
	if cc.inGpuMethodCount < 0 {
		panic("CheckContext: negative gpu method count")
	}

	if cc.inCpuMethodCount > 0 && cc.inGpuMethodCount > 0 {
		panic("CheckContext: both cpu and gpu counts positive")
	}
}

func (cc *CheckContext) EnterGpuBlock() {
	cc.inGpuMethodCount += 1
}

func (cc *CheckContext) LeaveGpuBlock() {
	cc.inGpuMethodCount -= 1
	cc.validateCpuGpuCount()
}

func (cc *CheckContext) EnterCpuBlock() {
	cc.inCpuMethodCount += 1
}

func (cc *CheckContext) LeaveCpuBlock() {
	cc.inCpuMethodCount -= 1
	cc.validateCpuGpuCount()
}

/*
Call this in any pass where Cpu is required

It will add an error if cpu is not available
This can be called in any pass
*/
func (cc *CheckContext) NoteCpuRequired(msg string) {
	if cc.inCpuMethodCount == 0 {
		cc.Errors.Errorf("CPU is required for this statement: %v", msg)
	}
}

/*
Call this in any pass where Gpu is required

It will add an error if gpu is not available
This can be called in any pass
*/
func (cc *CheckContext) NoteGpuRequired(msg string) {
	if cc.inGpuMethodCount == 0 {
		cc.Errors.Errorf("GPU is required for this statement: %v", msg)
	}
}

/*
The current return type pushed by PushReturnType()
*/
func (cc *CheckContext) CurrentReturnType() (Type, bool) {
	if len(cc.returnTypes) == 0 {
		return Type{}, false
	}

	return cc.returnTypes[len(cc.returnTypes)-1], true
}

/*
Add a statement to be inserted before this one
*/
func (cc *CheckContext) InsertStatementBefore(s Statement) {
	i := len(cc.insertStatements) - 1
	cc.insertStatements[i] = append(cc.insertStatements[i], s)
}

/*
Add an element to be inserted before this one
*/
func (cc *CheckContext) InsertElementBefore(tle TopLevelElement) {
	cc.insertElements = append(cc.insertElements, tle)
}

/*
Call this to ensure the current statement is evicted after the check stage
*/
func (cc *CheckContext) RemoveThisStatement() {
	cc.shouldRemoveStatement = true
}

/*
This will return true once (and clear) when the current statement should be removed
*/
func (cc *CheckContext) ShouldRemoveStatement() bool {
	r := cc.shouldRemoveStatement
	cc.shouldRemoveStatement = false
	return r
}

func (cc *CheckContext) StartStatementCollectionBlock() {
	cc.insertStatements = append(cc.insertStatements, []Statement{})
}

/*
Get any statements that should be inserted before this one
*/
func (cc *CheckContext) StopStatementCollectionBlock() []Statement {
	i := len(cc.insertStatements) - 1
	stmts := cc.insertStatements[i]
	cc.insertStatements = cc.insertStatements[:i]

	if len(stmts) == 0 {
		return nil
	} else {
		return stmts
	}
}

/*
Get any statements that should be inserted before this one
*/
func (cc *CheckContext) GetElementsForInsert() []TopLevelElement {
	if len(cc.insertElements) == 0 {
		return nil
	}

	r := cc.insertElements
	cc.insertElements = []TopLevelElement{}
	return r
}

/*
Set the return type on this context
*/
func (cc *CheckContext) PushReturnType(ty Type) {
	cc.returnTypes = append(cc.returnTypes, ty)
}

/*
Pop a return type
*/
func (cc *CheckContext) PopReturnType() {
	cc.returnTypes = cc.returnTypes[:(len(cc.returnTypes) - 1)]
}

func (cc *CheckContext) CurrentPass() CheckPass {
	return cc.Pass
}

// Name for a field in a tuple
func TupleFieldName(i int) string {
	return fmt.Sprintf("f%v", i)
}

func (cc *CheckContext) PrepareForPass(mod *Module) {
	// clear out old structs from previous passes
	cc.Structs = []*RequiredStruct{}
	cc.Vectors = map[string]Type { }
	cc.currentModule = mod
}

func (cc *CheckContext) RequireVector(ty Type, scope *Scope) {
	id := ty.RawIdentifier()
	if _, fnd := cc.Vectors[id]; fnd {
		return
	}

	cc.Vectors[id] = ty

	scope.AddCFunction(CFunction {
		Name: ty.VectorAddName(),
		ReturnType: MakeVoid(),
		ArgumentTypes: []Type { ty },
	})
}

func (cc *CheckContext) RequireType(ty Type, scope *Scope) {
	if ty.Selector == KTypeFloat && ty.Width != 32 {
		cc.NoteCpuRequired("64 bit float")
	}

	if ty.Selector == KTypeTuple || ty.Selector == KTypeStruct {
		tyid := ty.TupleIdentifier()
		for _, rs := range cc.Structs {
			if rs.TypeId == tyid {
				return
			}
		}

		switch ty.Selector {
		case KTypeTuple:
			// construct the struct
			defn := StructDefinition{
				Fields: []StructField{},
			}

			for i, ty := range ty.Types {
				defn.Fields = append(defn.Fields, StructField{
					Name: TupleFieldName(i),
					Type: ty,
				})
			}

			cc.Structs = append(cc.Structs, &RequiredStruct{
				GeneratedForTuple: true,
				TypeId:            tyid,
				Id: StructId{
					Module: cc.currentModule.Id,
					Name:   tyid,
				},
				Definition: defn,
			})

		case KTypeStruct:
			sd, fnd := scope.LookupStructDefinition(ty.StructId)
			if !fnd {
				cc.Errors.Errorf("Failed to find struct definition for '%v'", ty.StructId)
			}
			cc.Structs = append(cc.Structs, &RequiredStruct{
				GeneratedForTuple: false,
				TypeId:            tyid,
				Id:                ty.StructId,
				Definition:        sd,
			})
		}
	}
}

func functionReturning(ty Type) Type {
	return Type{
		Selector: KTypeFunction,
		Return:   &ty,
	}
}

func voidFunction() Type {
	return functionReturning(Type{Selector: KTypeVoid})
}
