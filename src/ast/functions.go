package ast

import (
	"bytes"
	"fmt"
	"sort"
)

// A fully resolved function identifier
type FunctionId struct {
	// the module that owns this
	Module ModuleId

	// The struct/etc within the module that owns this
	// This is blank if it is unbound to a struct
	Struct StructId

	// the user facing name
	Name string
}

func (fi FunctionId) String() string {
	return fmt.Sprintf("FunctionId(%v, %v, %v)", fi.Module.Key(), fi.Struct, fi.Name)
}

func (lhs FunctionId) IsEqual(rhs FunctionId) bool {
	if lhs.Name != rhs.Name {
		return false
	}

	if !lhs.Struct.IsEqual(rhs.Struct) {
		return false
	}

	if !lhs.Module.IsEqual(rhs.Module) {
		return false
	}

	return true
}

type FunctionLocation int

const (
	/*
	   This needs to run on the CPU

	   Eventually this should probably be split into actual requirements
	   Likely some cpu functions are available on gpu, and some will become so eventually
	*/
	KLocationCpu FunctionLocation = iota

	/*
	   Pure code that can run anywhere
	*/
	KLocationAnywhere

	/*
	   This needs to run on the GPU
	*/
	KLocationGpu
)

func FunctionLocations() []FunctionLocation {
	return []FunctionLocation {
		KLocationCpu,
		KLocationGpu,
		KLocationAnywhere,
	}
}

type FunctionSignature struct {
	Location FunctionLocation
	Return   Type
	Types    []Type
}

/*
   An id of type and location
 */
func (fs FunctionSignature) MapKey() string {
	buf := bytes.NewBuffer([]byte{})
	fmt.Fprintf(buf, fs.Return.RawIdentifier())
	fmt.Fprintf(buf, "__")
	for tyi, ty := range fs.Types {
		if tyi > 0 {
			fmt.Fprintf(buf, "_")
		}
		fmt.Fprintf(buf, ty.RawIdentifier())
	}
	return buf.String()
}

/*
   complete grouping of functions of a specific signature, along with type
 */
type FunctionSet struct {
	// Common signature for all in the set
	Signature FunctionSignature

	// A set of function ids
	AllIds map[FunctionLocation][]FunctionId
}

func NewFunctionSet(sig FunctionSignature) *FunctionSet {
	fs := &FunctionSet{
		Signature: sig,
		AllIds:    map[FunctionLocation][]FunctionId{},
	}

	for _, loc := range FunctionLocations() {
		fs.AllIds[loc] = []FunctionId {}
	}

	return fs
}

func (fs *FunctionSet) UniqueIds() []FunctionId {
	fmt.Println("UniqueIds")
	mp := map[string]FunctionId {}

	for _, ids := range fs.AllIds {
		for _, id := range ids {
			mp[id.String()] = id
			fmt.Println("id = ", id)
		}
	}

	ret := []FunctionId {}
	for _, id := range mp {
		ret = append(ret, id)
		fmt.Println("out = ", id)
	}
	return ret
}

func (fs *FunctionSet) MergeIn(ofs *FunctionSet) {
	if fs.Signature.MapKey() != ofs.Signature.MapKey() {
		panic(fmt.Sprintf("FunctionSet.MergeIn: Map keys do not match %v != %v", fs.Signature.MapKey(), ofs.Signature.MapKey()))
	}

	for loc, ids := range ofs.AllIds {
		for _, id := range ids {
			fs.AllIds[loc] = append(fs.AllIds[loc], id)
		}
	}
}

type FunctionGroup struct {
	// functions indexed by FunctionSignature.MapKey()
	Functions map[string]*FunctionSet
}

type FunctionEntry struct {
	// A global integer id, same on c/gpu
	// This must be consistent or bugs arise in the closures
	Id int

	// The function id 
	Fid FunctionId

	// Where this function entry is located
	Location FunctionLocation
}

func (fg *FunctionGroup) FunctionEntries() []FunctionEntry {
	tlKeys := []string { }
	for k, _ := range fg.Functions {
		tlKeys = append(tlKeys, k)
	}
	sort.Strings(tlKeys)

	runningId := 0
	fes := []FunctionEntry {}

	for _, k := range tlKeys {
		fs := fg.Functions[k]

		for _, loc := range FunctionLocations() {
			ids := fs.AllIds[loc]
			for _, fid := range ids {
				fes = append(fes, FunctionEntry {
					Id: runningId,
					Location: loc,
					Fid: fid,
				})
				runningId += 1
			}
		}
	}

	return fes
}

/*
The most args passed by any
*/
func (fg *FunctionGroup) MaxArgCount() int {
	maxArgCount := 0

	for _, fs := range fg.Functions {
		argCount := len(fs.Signature.Types)
		if argCount > maxArgCount {
			maxArgCount = argCount
		}
	}
		
	return maxArgCount
}

func NewFunctionGroup() *FunctionGroup {
	return &FunctionGroup {
		Functions: map[string]*FunctionSet {},
	}
}

func (fg *FunctionGroup) MergeIn(mfg *FunctionGroup) {
	for k, v := range mfg.Functions {
		fs, fsExists := fg.Functions[k]
		if !fsExists {
			fg.Functions[k] = v
		} else {
			fs.MergeIn(v)
		}
	}
}

func (fg *FunctionGroup) Add(id FunctionId, fs FunctionSignature, loc FunctionLocation) {
	key := fs.MapKey()

	fsv, ok := fg.Functions[key]
	if !ok {
		fsv = NewFunctionSet(fs)
	}

	/*
	   Struct functions seem to end up in twice, this deduplicates them
	*/
	exists := false
	for _, fid := range fsv.AllIds[loc] {
		if fid.IsEqual(id) {
			exists = true
			break
		}
	}
	if !exists {
		fsv.AllIds[loc] = append(fsv.AllIds[loc], id)
	}

	fg.Functions[key] = fsv
}

type FunctionParameter struct {
	Name string
	Type Type
}

type FunctionDefinition struct {
	Id              FunctionId
	Return          Type
	AvoidCheckPhase bool
	Location        FunctionLocation

	// scope including function parameters
	Scope *Scope

	Block      *StatementBlock
	Parameters []FunctionParameter
}

func (fd *FunctionDefinition) String() string {
	return fmt.Sprintf("FunctionDefinition(%v)", fd.Id)
}

func (fd *FunctionDefinition) Signature() FunctionSignature {
	types := []Type{}

	if !fd.Id.Struct.Blank() {
		types = append(types, MakePointer(Type{Selector: KTypeStruct, StructId: fd.Id.Struct}))
	}

	for _, fp := range fd.Parameters {
		types = append(types, fp.Type)
	}

	return FunctionSignature{
		Return:   fd.Return,
		Types:    types,
		Location: fd.Location,
	}
}

// the type of this function when viewed as a variable
func (fd *FunctionDefinition) OurType() Type {
	ftype := Type{
		Return:   &fd.Return,
		Selector: KTypeFunction,
		Types:    []Type{},
		Location: fd.Location,
	}

	for _, arg := range fd.Parameters {
		ftype.Types = append(ftype.Types, arg.Type)
	}

	return ftype
}

func CheckStatementBlockEndsWithReturn(sb *StatementBlock) bool {
	if len(sb.Statements) == 0 {
		return false
	} else {
		last := sb.Statements[len(sb.Statements)-1].Statement

		switch s := last.(type) {
		case *ReturnStatement:
			return true

		case *IfStatement:
			for _, seg := range s.Segments {
				if !CheckStatementBlockEndsWithReturn(seg.Block) {
					return false
				}
			}
			return true

		default:
			return false
		}
	}
}

func (fd *FunctionDefinition) Check(ctx *CheckContext, externalScope *Scope) {
	if fd.Id.Module.Blank() {
		panic("blank definition " + fd.Id.Name)
	}

	switch fd.Location {
	case KLocationGpu:
		ctx.EnterGpuBlock()
		defer ctx.LeaveGpuBlock()

	case KLocationCpu:
		ctx.EnterCpuBlock()
		defer ctx.LeaveCpuBlock()
	}

	switch ctx.CurrentPass() {
	case KPassSetTypes:
		if fd.Return.Selector != KTypeVoid {
			if !CheckStatementBlockEndsWithReturn(fd.Block) {
				ctx.Errors.Errorf("A non-void function must end with a return")
				return
			}
		}

		for _, arg := range fd.Parameters {
			ctx.RequireType(arg.Type, externalScope)
		}

		// struct functions should not be readily accessible in the local namespace
		if fd.Id.Struct.Blank() {
			externalScope.SetVariable(fd.Id.Name, fd.OurType(), false)
		}

		ctx.RequireType(fd.Return, externalScope)
		if !ctx.Errors.Clean() {
			return
		}
		ctx.Functions.Add(fd.Id, fd.Signature(), fd.Location)

	case KPassCheckTypes:
		if fd.AvoidCheckPhase {
			return
		}
	}

	ctx.PushReturnType(fd.Return)
	defer ctx.PopReturnType()
	fd.Block.Check(ctx)
}

// Return the effective parameters of a function, NB this takes into account whether or not it is bound to a struct
func (fd FunctionDefinition) EffectiveParameters(executionContextParameter FunctionParameter) []FunctionParameter {
	prms := fd.Parameters

	if !fd.Id.Struct.Blank() {
		iprms := []FunctionParameter{
			FunctionParameter{
				Name: "ey_self",
				Type: Type{
					Selector: KTypePointer,
					Types: []Type{
						Type{
							Selector: KTypeStruct,
							StructId: fd.Id.Struct,
						},
					},
				},
			},
		}
		prms = append(iprms, prms...)
	}

	iprms := []FunctionParameter{
		executionContextParameter,
	}
	prms = append(iprms, prms...)

	return prms
}

