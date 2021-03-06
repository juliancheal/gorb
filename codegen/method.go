package codegen

import (
	"fmt"
	"strings"
	"text/template"
)

type scope int

const (
	instanceScope scope = 0
	classScope          = 1
)

type method struct {
	g           *Generator
	class       *class
	indirection int
	returnClass string
	ctor        bool
	name        string
	exportName  string
	scope       scope
	args        []string
	argTypes    []string
	returnTypes []string

	blockArgs        []string
	blockArgTypes    []string
	blockReturnTypes []string
}

func (m *method) HasBlock() bool {
	return len(m.blockArgs) > 0
}

func (m *method) HasBlockReturnType() bool {
	return len(m.blockReturnTypes) > 0
}

func (m *method) BlockSig() string {
	args := []string{}
	for i, n := range m.blockArgs {
		args = append(args, n+" "+m.blockArgTypes[i])
	}

	return fmt.Sprintf("block__%s(%s) (%s)", m.FuncName(),
		strings.Join(args, ", "), strings.Join(m.blockReturnTypes, ", "))
}

func (m *method) RbBlockArgs() string {
	outargs := make([]string, len(m.blockArgs))
	for i, a := range m.blockArgs {
		prefix := ""
		if m.g.isValueType(m.blockArgTypes[i]) {
			prefix = "*"
		}
		outargs[i] = prefix + "rb_" + a
	}
	return strings.Join(outargs, ", ")
}

func (m *method) BlockArgsList() []string {
	return m.blockArgs
}

func (m *method) BlockArgToRb(n int) string {
	ret := m.blockArgs[n]
	if r := m.g.resolvedType(m.blockArgTypes[n]); r != "" && isExported(r) {
		var v string
		if class := m.g.findClass(r); class != nil {
			v = class.VarName()
		} else {
			v = "rb_cObject"
		}
		if m.indirection == 0 {
			ret = "&" + ret
		}
		return fmt.Sprintf("gorb.StructValue(%s, unsafe.Pointer(%s))", v, ret)
	}

	if r := m.g.resolvedType(m.blockArgTypes[n]); isExternal(r) {
		if m.indirection == 0 {
			ret = "&" + ret
		}

		m.g.pkg.usedImports[m.g.pkg.imports[packageName(r)]] = true
		return fmt.Sprintf("gorb.StructValue(gorb.ObjAtPath(\"%s\"), unsafe.Pointer(%s))",
			m.g.importToModule(r), ret)
	}

	t, _ := m.g.returnTypes(m.blockArgTypes[n])
	return fmt.Sprintf("gorb.%s(%s))", t[0], ret)
}

func (m *method) BlockReceiverVars() string {
	if m.blockReturnTypes[len(m.blockReturnTypes)-1] == "error" {
		return "ret, err"
	}
	return "ret"
}

func (m *method) BlockReturnTypeToGo() string {
	return m.typeToGo(m.blockReturnTypes[0], "ret")
}

func (m *method) ResolvedReturnType() string {
	if len(m.returnTypes) == 0 {
		return "nil"
	}
	return m.g.resolvedType(m.returnTypes[0])
}

func (m *method) ResolvedReturnClass() string {
	return m.g.resolvedType(m.returnClass)
}

func (m *method) ClassName() string {
	return m.class.Name()
}

func (m *method) ClassVar() string {
	return m.class.VarName()
}

func (m *method) Name() string {
	return capitalize(m.name)
}

func (m *method) RubyName() string {
	name := underscore(m.name)
	if name == "string" {
		return "to_s"
	}
	if m.ResolvedReturnType() == "bool" {
		name += "?"
	}
	return name
}

func (m *method) ExportRubyName() string {
	if m.exportName != "" {
		return m.exportName
	}
	return m.RubyName()
}

func (m *method) FuncName() string {
	s := "i"
	if m.scope == classScope {
		s = "c"
	}
	prefix := "g_" + s + "method_"
	if m.class != nil {
		prefix += m.class.Name()
	}
	return prefix + "_" + m.name
}

func (m *method) Scope() string {
	if m.scope == classScope {
		return "Class"
	}
	return ""
}

func (m *method) RubyArgs() string {
	return strings.Join(append([]string{"self"}, m.args...), ", ")
}

func (m *method) GoArgs() string {
	outargs := make([]string, len(m.args))
	for i, a := range m.args {
		prefix := ""
		if m.g.isValueType(m.argTypes[i]) {
			prefix = "*"
		}
		outargs[i] = prefix + "go_" + a
	}
	if m.HasBlock() {
		outargs = append(outargs, "block__"+m.FuncName())
	}
	return strings.Join(outargs, ", ")
}

func (m *method) ArgsList() []string {
	return m.args
}

func (m *method) ArgToGo(n int) string {
	return m.typeToGo(m.argTypes[n], m.args[n])
}

func (m *method) ReceiverVars() string {
	if m.returnTypes[len(m.returnTypes)-1] == "error" {
		return "ret, err"
	}
	return "ret"
}

func (m *method) RaiseError() string {
	if len(m.returnTypes) > 0 && m.ReceiverVars() != "ret" {
		return "\n  gorb.RaiseError(err)"
	}
	return ""
}

func (m *method) ReturnTypeToRuby() string {
	if len(m.returnTypes) == 0 {
		return "C.Qnil"
	}
	ret := "ret"
	if r := m.ResolvedReturnClass(); r != "" && isExported(r) {
		var v string
		if class := m.g.findClass(r); class != nil {
			v = class.VarName()
		} else {
			v = "rb_cObject"
		}
		if m.indirection == 0 {
			ret = "&" + ret
		}
		return fmt.Sprintf("gorb.StructValue(%s, unsafe.Pointer(%s))", v, ret)
	}

	if r := m.ResolvedReturnType(); isExternal(r) {
		if m.indirection == 0 {
			ret = "&" + ret
		}

		m.g.pkg.usedImports[m.g.pkg.imports[packageName(r)]] = true
		return fmt.Sprintf("gorb.StructValue(gorb.ObjAtPath(\"%s\"), unsafe.Pointer(%s))",
			m.g.importToModule(r), ret)
	}

	t, _ := m.g.returnTypes(m.ResolvedReturnType())
	return fmt.Sprintf("gorb.%s(%s))", t[0], ret)
}

func (m *method) typeToGo(typ string, val string) string {
	t, _ := m.g.returnTypes(typ)
	out := "gorb." + t[1] + "(" + val + ")"
	v := typ
	if isExported(v) {
		for v != "" {
			if m.g.revTypeAliasMap[v] == "" {
				break
			}
			v = m.g.revTypeAliasMap[v]
		}
		v = insertPkg(v, m.g.pkg.name)
	}

	if t[1] == "GoStruct" {
		if indirection(v) == 0 {
			v = "*" + v
		}
		out = fmt.Sprintf("(%s)(%s)", v, out)
	} else {
		out = fmt.Sprintf("%s(%s)", v, out)
	}
	out = strings.Join(make([]string, m.indirection), "&") + out
	return out
}

func (m *method) ReturnTypeToGo() string {
	return m.typeToGo(m.returnTypes[0], "val")
}

func (m *method) HasReturnType() bool {
	return len(m.returnTypes) > 0
}

func (m *method) FnReceiver() string {
	if m.scope == classScope {
		return m.g.pkg.name
	}
	return "go_obj"
}

func (m *method) RubyEnumArgs() string {
	return strings.Join(append([]string{"self",
		"gorb.StringValue(" + fmt.Sprintf("%q", m.RubyName()) + ")"},
		m.args...), ", ")
}

const tplMethodData = `
//export {{.FuncName}}
func {{.FuncName}}({{.RubyArgs}} uintptr) uintptr {
{{- if .HasBlock}}
	if e := gorb.EnumFor({{.RubyEnumArgs}}); e != C.Qnil {
		return e
	}

{{- end}}
{{- if ne .Scope "Class"}}
	{{.FnReceiver}} := g_val2ptr_{{.ClassName}}(self)
{{- end}}
{{- range $i, $v := .ArgsList}}
	go_{{$v}} := {{$.ArgToGo $i}}
{{- end}} 
{{- if .HasReturnType}}
	{{.ReceiverVars}} := {{.FnReceiver}}.{{.Name}}({{.GoArgs}}){{.RaiseError}}
	return {{.ReturnTypeToRuby}}
{{- else}}
	{{.FnReceiver}}.{{.Name}}({{.GoArgs}}){{.RaiseError}}
	return C.Qnil
{{- end}}
}
{{- if .HasBlock}}

func {{.BlockSig}} {
{{- range $i, $v := .BlockArgsList}}
	rb_{{$v}} := {{$.BlockArgToRb $i}}
{{- end}}
{{- if .HasBlockReturnType}}
	ret := gorb.Yield({{.RbBlockArgs}})
	return {{.BlockReturnTypeToGo}}
{{- else}}
	gorb.Yield({{.RbBlockArgs}})
{{- end}}
}
{{- end}}

`

var tplMethod = template.Must(template.New("method").Parse(tplMethodData))

func (m *method) write(g *Generator) {
	g.writePreambleFunc(m.FuncName(), len(m.args))

	if m.scope == classScope && m.ctor == false { // module function
		fmt.Fprintf(&g.init, `	gorb.DefineModuleFunction(g_pkg, "%s", C.%s, %d)`+"\n",
			m.RubyName(), m.FuncName(), len(m.args))
	} else {
		fmt.Fprintf(&g.init, `	gorb.Define%sMethod(%s, "%s", C.%s, %d)`+"\n",
			m.Scope(), m.class.VarName(), m.ExportRubyName(), m.FuncName(), len(m.args))
	}
	if err := tplMethod.Execute(&g.methods, m); err != nil {
		panic(err)
	}
}
