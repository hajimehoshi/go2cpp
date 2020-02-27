// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/go-interpreter/wagon/wasm"
	"golang.org/x/tools/go/packages"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func identifierFromString(str string) string {
	var ident string
	for _, r := range []rune(str) {
		if r > 0xff {
			panic("identifiers cannot include non-Latin1 characters")
		}
		if '0' <= r && r <= '9' {
			ident += string(r)
			continue
		}
		if 'a' <= r && r <= 'z' {
			ident += string(r)
			continue
		}
		if 'A' <= r && r <= 'Z' {
			ident += string(r)
			continue
		}
		ident += fmt.Sprintf("_%02x", r)
	}
	return ident
}

func namespaceFromPkg(pkg *packages.Package) string {
	ts := strings.Split(pkg.PkgPath, "/")
	for i, t := range ts {
		ts[i] = identifierFromString(t)
	}
	return strings.Join(ts, ".")
}

type Func struct {
	Sig   *wasm.FunctionSig
	Body  *wasm.FunctionBody
	Index int
	Name  string
}

var funcTmpl = template.Must(template.New("func").Parse(`// OriginalName: {{.OriginalName}}
// Index:        {{.Index}}
internal {{.ReturnType}} {{.Name}}({{.Args}})
{
{{range .Locals}}    {{.}}
{{end}}
{{range .Body}}    {{.}}
{{end}}
{{if ne .ReturnType "void"}}    return 0;
{{end}}}`))

func wasmTypeToCSharpType(v wasm.ValueType) string {
	switch v {
	case wasm.ValueTypeI32:
		return "int"
	case wasm.ValueTypeI64:
		return "long"
	case wasm.ValueTypeF32:
		return "float"
	case wasm.ValueTypeF64:
		return "double"
	default:
		panic("not reached")
	}
}

func (f *Func) CSharp(indent string) (string, error) {
	var retType string
	switch ts := f.Sig.ReturnTypes; len(ts) {
	case 0:
		retType = "void"
	case 1:
		retType = wasmTypeToCSharpType(ts[0])
	default:
		panic("the number of return values should be 0 or 1 so far")
	}

	var args []string
	for i, t := range f.Sig.ParamTypes {
		args = append(args, fmt.Sprintf("%s arg%d", wasmTypeToCSharpType(t), i))
	}

	var locals []string
	var body []string
	if f.Body != nil {
		var idx int
		for _, e := range f.Body.Locals {
			for i := 0; i < int(e.Count); i++ {
				locals = append(locals, fmt.Sprintf("%s local%d;", wasmTypeToCSharpType(e.Type), idx))
				idx++
			}
		}
		var err error
		body, err = opsToCSharp(f.Body.Code)
		if err != nil {
			return "", err
		}
	}

	var buf bytes.Buffer
	if err := funcTmpl.Execute(&buf, struct {
		OriginalName string
		Name         string
		Index        int
		ReturnType   string
		Args         string
		Locals       []string
		Body         []string
	}{
		OriginalName: f.Name,
		Name:         identifierFromString(f.Name),
		Index:        f.Index,
		ReturnType:   retType,
		Args:         strings.Join(args, ", "),
		Locals:       locals,
		Body:         body,
	}); err != nil {
		return "", err
	}

	// Add indentations
	var lines []string
	for _, line := range strings.Split(buf.String(), "\n") {
		lines = append(lines, indent+line)
	}
	return strings.Join(lines, "\n") + "\n", nil
}

type Global struct {
	Type  wasm.ValueType
	Index int
	Init  int
}

func (g *Global) CSharp(indent string) string {
	return fmt.Sprintf("%s%s global%d = %d;", indent, wasmTypeToCSharpType(g.Type), g.Index, g.Init)
}

func run() error {
	tmp, err := ioutil.TempDir("", "go2dotnet-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	wasmpath := filepath.Join(tmp, "tmp.wasm")

	// TODO: Detect the last argument is path or not
	pkgname := os.Args[len(os.Args)-1]

	args := append([]string{"build"}, os.Args[1:]...)
	args = append(args[:len(args)-1], "-o="+wasmpath, pkgname)
	cmd := exec.Command("go", args...)
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	if err := cmd.Run(); err != nil {
		return err
	}

	f, err := os.Open(wasmpath)
	if err != nil {
		return err
	}
	defer f.Close()

	mod, err := wasm.ReadModule(f, nil)
	if err != nil {
		return err
	}

	var ifs []*Func
	var fs []*Func
	for i, f := range mod.FunctionIndexSpace {
		// There is a bug that signature and body are shifted (go-interpreter/wagon#190).
		// TODO: Avoid using FunctionIndexSpace?
		if f.Name == "" {
			ifs = append(ifs, &Func{
				Sig:   &mod.Types.Entries[mod.Import.Entries[i].Type.(wasm.FuncImport).Type],
				Index: i,
				Name:  mod.Import.Entries[i].FieldName,
			})
			continue
		}

		f2 := mod.FunctionIndexSpace[i - len(mod.Import.Entries)]
		fs = append(fs, &Func{
			Sig:   f2.Sig,
			Body:  f2.Body,
			Index: i,
			Name:  f.Name,
		})
	}

	var globals []*Global
	for i, e := range mod.Global.Globals {
		// TODO: Consider mutability.
		// TODO: Use e.Type.Init.
		globals = append(globals, &Global{
			Type:  e.Type.Type,
			Index: i,
			Init:  0,
		})
	}

	pkgs, err := packages.Load(nil, pkgname)
	if err != nil {
		return err
	}

	namespace := namespaceFromPkg(pkgs[0])
	class := identifierFromString(pkgs[0].Name)

	if err := csTmpl.Execute(os.Stdout, struct {
		Namespace   string
		Class       string
		ImportFuncs []*Func
		Funcs       []*Func
		Globals     []*Global
	}{
		Namespace:   namespace,
		Class:       class,
		ImportFuncs: ifs,
		Funcs:       fs,
		Globals:     globals,
	}); err != nil {
		return err
	}

	return nil
}

var csTmpl = template.Must(template.New("out.cs").Parse(`// Code generated by go2dotnet. DO NOT EDIT.

// TODO: Remove this.
#pragma warning disable 168
#pragma warning disable 414

using System.Diagnostics;

namespace {{.Namespace}}
{
    sealed class Import
    {
{{- range $value := .ImportFuncs}}
{{$value.CSharp "        "}}{{end}}    }

    sealed class Go_{{.Class}}
    {
{{- range $value := .Globals}}
{{$value.CSharp "        "}}{{end}}
{{range $value := .Funcs}}
{{$value.CSharp "        "}}{{end}}    }
}
`))
