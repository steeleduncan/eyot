package textwriter

import (
	"fmt"
	"io"
)

type lineComponent struct {
	Content string
	Space   bool
}

type W struct {
	w                   io.Writer
	cpts                []lineComponent
	indent              int
	noSpace, forceSpace bool
}

func NewWriter(w io.Writer) *W {
	return &W{
		w:          w,
		cpts:       []lineComponent{},
		indent:     0,
		noSpace:    false,
		forceSpace: false,
	}
}

func (w *W) ForceSpace() {
	w.forceSpace = true
}

func (w *W) SuppressNextSpace() {
	w.noSpace = true
}

func (w *W) addComponent(cpt lineComponent) {
	if w.forceSpace {
		cpt.Space = true
	} else if w.noSpace {
		cpt.Space = false
	}
	w.noSpace = false
	w.forceSpace = false
	w.cpts = append(w.cpts, cpt)
}

func (w *W) AddComponent(s string) {
	cpt := lineComponent{Content: s, Space: true}
	w.addComponent(cpt)
}

func (w *W) AddComponents(args ...interface{}) {
	for _, arg := range args {
		argString, ok := arg.(string)
		if !ok {
			panic("Must pass strings")
		}

		w.AddComponent(argString)
	}
}

func (w *W) AddComponentNoSpace(s string) {
	cpt := lineComponent{Content: s, Space: false}
	w.addComponent(cpt)
}

func (w *W) AddComponentf(format string, args ...interface{}) {
	w.AddComponent(fmt.Sprintf(format, args...))
}

func (w *W) Indent() {
	w.indent += 1
}

func (w *W) Unindent() {
	w.indent -= 1
}

func (w *W) WriteRaw(src string) {
	fmt.Fprint(w.w, src)
}

func (w *W) EndLine() {
	if len(w.cpts) == 0 {
		fmt.Fprintln(w.w)
		return
	}

	for i := 0; i < w.indent; i += 1 {
		fmt.Fprint(w.w, "    ")
	}

	for i, cpt := range w.cpts {
		if i > 0 && cpt.Space {
			fmt.Fprint(w.w, " ")
		}

		fmt.Fprint(w.w, cpt.Content)
	}

	fmt.Fprintln(w.w)
	w.cpts = []lineComponent{}
}
