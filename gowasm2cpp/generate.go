// SPDX-License-Identifier: Apache-2.0

package gowasm2cpp

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/go-interpreter/wagon/wasm"
	"golang.org/x/sync/errgroup"
)

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
	if len(ident) > 512 {
		ident = ident[:511]
	}
	return ident
}

func includeGuard(str string) string {
	return strings.ToUpper(str)
}

type wasmFunc struct {
	Mod     *wasm.Module
	Funcs   []*wasmFunc
	Types   []*wasmType
	Globals []*wasmGlobal
	Type    *wasmType
	Wasm    wasm.Function
	Index   int
	Import  bool
	BodyStr string
}

func (f *wasmFunc) Identifier() string {
	return identifierFromString(f.Wasm.Name)
}

var funcDeclTmpl = template.Must(template.New("funcDecl").Parse(`// OriginalName: {{.OriginalName}}
// Index:        {{.Index}}
{{if .Abstract}}virtual {{end}}{{.ReturnType}} {{.Name}}({{.Args}}){{if .Abstract}} = 0{{end}}{{if .Override}} override{{end}};`))

var funcImplTmpl = template.Must(template.New("func").Parse(`// OriginalName: {{.OriginalName}}
// Index:        {{.Index}}
{{.ReturnType}} {{.Class}}::{{.Name}}({{.Args}}) {
{{range .Locals}}  {{.}}
{{end}}{{if .Locals}}
{{end}}{{range .Body}}{{.}}
{{end}}}`))

func wasmTypeToReturnType(v wasm.ValueType) returnType {
	switch v {
	case wasm.ValueTypeI32:
		return returnTypeI32
	case wasm.ValueTypeI64:
		return returnTypeI64
	case wasm.ValueTypeF32:
		return returnTypeF32
	case wasm.ValueTypeF64:
		return returnTypeF64
	default:
		panic("not reached")
	}
}

func (f *wasmFunc) CppDecl(indent string, abstract bool, override bool) (string, error) {
	var retType returnType
	switch ts := f.Wasm.Sig.ReturnTypes; len(ts) {
	case 0:
		retType = returnTypeVoid
	case 1:
		retType = wasmTypeToReturnType(ts[0])
	default:
		return "", fmt.Errorf("the number of return values must be 0 or 1 but %d", len(ts))
	}

	var args []string
	for i, t := range f.Wasm.Sig.ParamTypes {
		args = append(args, fmt.Sprintf("%s local%d", wasmTypeToReturnType(t).Cpp(), i))
	}

	var buf bytes.Buffer
	if err := funcDeclTmpl.Execute(&buf, struct {
		OriginalName string
		Name         string
		Index        int
		ReturnType   string
		Args         string
		Abstract     bool
		Override     bool
	}{
		OriginalName: f.Wasm.Name,
		Name:         identifierFromString(f.Wasm.Name),
		Index:        f.Index,
		ReturnType:   retType.Cpp(),
		Args:         strings.Join(args, ", "),
		Abstract:     abstract,
		Override:     override,
	}); err != nil {
		return "", err
	}

	// Add indentations
	var lines []string
	for _, line := range strings.Split(buf.String(), "\n") {
		lines = append(lines, indent+line)
	}
	return strings.Join(lines, "\n"), nil
}

func (f *wasmFunc) CppImpl(className string, indent string) (string, error) {
	var retType returnType
	switch ts := f.Wasm.Sig.ReturnTypes; len(ts) {
	case 0:
		retType = returnTypeVoid
	case 1:
		retType = wasmTypeToReturnType(ts[0])
	default:
		return "", fmt.Errorf("the number of return values must be 0 or 1 but %d", len(ts))
	}

	var args []string
	for i, t := range f.Wasm.Sig.ParamTypes {
		args = append(args, fmt.Sprintf("%s local%d", wasmTypeToReturnType(t).Cpp(), i))
	}

	var locals []string
	var body []string
	if f.BodyStr != "" {
		body = strings.Split(f.BodyStr, "\n")
	} else if f.Wasm.Body != nil {
		idx := len(f.Wasm.Sig.ParamTypes)
		for _, e := range f.Wasm.Body.Locals {
			for i := 0; i < int(e.Count); i++ {
				locals = append(locals, fmt.Sprintf("%s local%d = 0;", wasmTypeToReturnType(e.Type).Cpp(), idx))
				idx++
			}
		}
		var err error
		body, err = f.bodyToCpp()
		if err != nil {
			return "", err
		}
		locals = removeUnusedLocalVariables(locals, body)
	} else {
		// TODO: Use error function.
		ident := identifierFromString(f.Wasm.Name)
		body = []string{
			fmt.Sprintf(`  std::cerr << "%s not implemented" << std::endl;`, ident),
			"  std::exit(1);"}
	}

	var buf bytes.Buffer
	if err := funcImplTmpl.Execute(&buf, struct {
		OriginalName string
		Name         string
		Class        string
		Index        int
		ReturnType   string
		Args         string
		Locals       []string
		Body         []string
	}{
		OriginalName: f.Wasm.Name,
		Name:         identifierFromString(f.Wasm.Name),
		Class:        className,
		Index:        f.Index,
		ReturnType:   retType.Cpp(),
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

var (
	localVariableRe = regexp.MustCompile(`local[0-9]+`)
)

func removeUnusedLocalVariables(decls []string, body []string) []string {
	decl2name := map[string]string{}
	for _, d := range decls {
		decl2name[d] = localVariableRe.FindString(d)
	}

	unused := map[string]struct{}{}
	for _, n := range decl2name {
		unused[n] = struct{}{}
	}

	for _, l := range body {
		for _, v := range localVariableRe.FindAllString(l, -1) {
			delete(unused, v)
		}
	}
	if len(unused) == 0 {
		return decls
	}

	r := make([]string, 0, len(decls) - len(unused))
	for _, d := range decls {
		v := decl2name[d]
		if _, ok := unused[v]; ok {
			continue
		}
		r = append(r, d)
	}
	return r
}

type wasmExport struct {
	Funcs []*wasmFunc
	Index int
	Name  string
}

func (e *wasmExport) CppDecl(indent string) (string, error) {
	f := e.Funcs[e.Index]

	var retType returnType
	switch ts := f.Wasm.Sig.ReturnTypes; len(ts) {
	case 0:
		retType = returnTypeVoid
	case 1:
		retType = wasmTypeToReturnType(ts[0])
	default:
		return "", fmt.Errorf("the number of return values must be 0 or 1 but %d", len(ts))
	}

	var args []string
	for i, t := range f.Wasm.Sig.ParamTypes {
		args = append(args, fmt.Sprintf("%s arg%d", wasmTypeToReturnType(t).Cpp(), i))
	}

	str := fmt.Sprintf(`%s %s(%s);`, retType.Cpp(), e.Name, strings.Join(args, ", "))

	lines := strings.Split(str, "\n")
	for i := range lines {
		lines[i] = indent + lines[i]
	}
	return strings.Join(lines, "\n"), nil
}

func (e *wasmExport) CppImpl(indent string) (string, error) {
	f := e.Funcs[e.Index]

	var ret string
	var retType returnType
	switch ts := f.Wasm.Sig.ReturnTypes; len(ts) {
	case 0:
		retType = returnTypeVoid
	case 1:
		ret = "return "
		retType = wasmTypeToReturnType(ts[0])
	default:
		return "", fmt.Errorf("the number of return values must be 0 or 1 but %d", len(ts))
	}

	var args []string
	var argsToPass []string
	for i, t := range f.Wasm.Sig.ParamTypes {
		args = append(args, fmt.Sprintf("%s arg%d", wasmTypeToReturnType(t).Cpp(), i))
		argsToPass = append(argsToPass, fmt.Sprintf("arg%d", i))
	}

	str := fmt.Sprintf(`%s Inst::%s(%s) {
  %s%s(%s);
}
`, retType.Cpp(), e.Name, strings.Join(args, ", "), ret, identifierFromString(f.Wasm.Name), strings.Join(argsToPass, ", "))

	lines := strings.Split(str, "\n")
	for i := range lines {
		lines[i] = indent + lines[i]
	}
	return strings.Join(lines, "\n"), nil
}

type wasmGlobal struct {
	Type  wasm.ValueType
	Index int
	Init  int
}

func (g *wasmGlobal) Cpp() string {
	return fmt.Sprintf("%s global%d = %d;", wasmTypeToReturnType(g.Type).Cpp(), g.Index, g.Init)
}

type wasmType struct {
	Sig   *wasm.FunctionSig
	Index int
}

func (t *wasmType) Cpp() (string, error) {
	var retType returnType
	switch ts := t.Sig.ReturnTypes; len(ts) {
	case 0:
		retType = returnTypeVoid
	case 1:
		retType = wasmTypeToReturnType(ts[0])
	default:
		return "", fmt.Errorf("the number of return values must be 0 or 1 but %d", len(ts))
	}
	var args []string
	for i, t := range t.Sig.ParamTypes {
		args = append(args, fmt.Sprintf("%s arg%d", wasmTypeToReturnType(t).Cpp(), i))
	}

	return fmt.Sprintf("%s (Inst::*)(%s)", retType.Cpp(), strings.Join(args, ", ")), nil
}

func Generate(outDir string, include string, wasmFile string, namespace string) error {
	f, err := os.Open(wasmFile)
	if err != nil {
		return err
	}
	defer f.Close()

	mod, err := wasm.DecodeModule(f)
	if err != nil {
		return err
	}

	var types []*wasmType
	for i, e := range mod.Types.Entries {
		e := e
		types = append(types, &wasmType{
			Sig:   &e,
			Index: i,
		})
	}

	var globals []*wasmGlobal
	for i, e := range mod.Global.Globals {
		// TODO: Consider mutability.
		// TODO: Use e.Type.Init.
		globals = append(globals, &wasmGlobal{
			Type:  e.Type.Type,
			Index: i,
			Init:  0,
		})
	}

	var ifs []*wasmFunc
	for i, e := range mod.Import.Entries {
		name := e.FieldName
		ifs = append(ifs, &wasmFunc{
			Type: types[e.Type.(wasm.FuncImport).Type],
			Wasm: wasm.Function{
				Sig:  types[e.Type.(wasm.FuncImport).Type].Sig,
				Name: name,
			},
			Globals: globals,
			Index:   i,
			Import:  true,
			BodyStr: importFuncBodies[name],
		})
	}

	// There is a bug that signature and body are shifted (go-interpreter/wagon#190).
	var names wasm.NameMap
	if c := mod.Custom(wasm.CustomSectionName); c != nil {
		var nsec wasm.NameSection
		if err := nsec.UnmarshalWASM(bytes.NewReader(c.Data)); err != nil {
			return err
		}
		if len(nsec.Types[wasm.NameFunction]) > 0 {
			sub, err := nsec.Decode(wasm.NameFunction)
			if err != nil {
				return err
			}
			names = sub.(*wasm.FunctionNames).Names
		}
	}
	var fs []*wasmFunc
	for i, t := range mod.Function.Types {
		name := names[uint32(i+len(mod.Import.Entries))]
		body := mod.Code.Bodies[i]
		fs = append(fs, &wasmFunc{
			Type: types[t],
			Wasm: wasm.Function{
				Sig:  types[t].Sig,
				Body: &body,
				Name: name,
			},
			Globals: globals,
			Index:   i + len(mod.Import.Entries),
		})
	}

	var exports []*wasmExport
	for _, e := range mod.Export.Entries {
		switch e.Kind {
		case wasm.ExternalFunction:
			exports = append(exports, &wasmExport{
				Index: int(e.Index),
				Name:  e.FieldStr,
			})
		case wasm.ExternalMemory:
			// Ignore
		default:
			return fmt.Errorf("export type %d is not implemented", e.Kind)
		}
	}

	allfs := append(ifs, fs...)
	for _, e := range exports {
		e.Funcs = allfs
	}
	for _, f := range ifs {
		f.Mod = mod
		f.Funcs = allfs
		f.Types = types
	}
	for _, f := range fs {
		f.Mod = mod
		f.Funcs = allfs
		f.Types = types
	}

	if mod.Start != nil {
		return fmt.Errorf("start section must be nil but not")
	}

	tables := make([][]uint32, len(mod.Table.Entries))
	for _, e := range mod.Elements.Entries {
		v, err := mod.ExecInitExpr(e.Offset)
		if err != nil {
			return err
		}
		offset := v.(int32)
		if diff := int(offset) + int(len(e.Elems)) - int(len(tables[e.Index])); diff > 0 {
			tables[e.Index] = append(tables[e.Index], make([]uint32, diff)...)
		}
		copy(tables[e.Index][offset:], e.Elems)
	}

	var data []wasmData
	for _, e := range mod.Data.Entries {
		offset, err := mod.ExecInitExpr(e.Offset)
		if err != nil {
			return err
		}
		data = append(data, wasmData{
			Offset: int(offset.(int32)),
			Data:   e.Data,
		})
	}

	var incpath string
	if include != "" {
		include = filepath.ToSlash(include)
		if include[len(include)-1] != '/' {
			include += "/"
		}
	}

	var g errgroup.Group
	g.Go(func() error {
		{
			out, err := os.Create(filepath.Join(outDir, "go.h"))
			if err != nil {
				return err
			}
			defer out.Close()

			if err := goHTmpl.Execute(out, struct {
				IncludeGuard string
				IncludePath  string
				Namespace    string
				ImportFuncs  []*wasmFunc
			}{
				IncludeGuard: includeGuard(namespace) + "_GO_H",
				IncludePath:  incpath,
				Namespace:    namespace,
				ImportFuncs:  ifs,
			}); err != nil {
				return err
			}
		}
		{
			out, err := os.Create(filepath.Join(outDir, "go.cpp"))
			if err != nil {
				return err
			}
			defer out.Close()

			if err := goCppTmpl.Execute(out, struct {
				IncludePath string
				Namespace   string
				ImportFuncs []*wasmFunc
			}{
				IncludePath: incpath,
				Namespace:   namespace,
				ImportFuncs: ifs,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	g.Go(func() error {
		return writeBits(outDir, incpath, namespace)
	})
	g.Go(func() error {
		return writeJS(outDir, incpath, namespace)
	})
	g.Go(func() error {
		return writeTaskQueue(outDir, incpath, namespace)
	})
	g.Go(func() error {
		return writeInst(outDir, incpath, namespace, ifs, fs, exports, globals, types, tables)
	})
	g.Go(func() error {
		return writeMem(outDir, incpath, namespace, int(mod.Memory.Entries[0].Limits.Initial), data)
	})

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

var goHTmpl = template.Must(template.New("go.h").Parse(`// Code generated by go2cpp. DO NOT EDIT.

#ifndef {{.IncludeGuard}}
#define {{.IncludeGuard}}

#include <algorithm>
#include <cstdint>
#include <chrono>
#include <functional>
#include <map>
#include <memory>
#include <stack>
#include <string>
#include <vector>
#include "{{.IncludePath}}js.h"
#include "{{.IncludePath}}inst.h"
#include "{{.IncludePath}}mem.h"
#include "{{.IncludePath}}taskqueue.h"

namespace {{.Namespace}} {

class Mem;
class Go;

class BindingValue {
public:
  BindingValue();
  explicit BindingValue(bool b);
  explicit BindingValue(double num);
  explicit BindingValue(const char* str);
  explicit BindingValue(const std::string& str);
  explicit BindingValue(const std::vector<uint8_t>& bytes);

  bool IsNull() const;
  bool IsUndefined() const;
  bool IsBool() const;
  bool IsNumber() const;
  bool IsString() const;
  bool IsBytes() const;
  bool IsFunction() const;

  bool ToBool() const;
  double ToNumber() const;
  std::string ToString() const;
  const std::vector<uint8_t>& ToBytes() const;

  BindingValue Invoke();
  BindingValue Invoke(std::vector<BindingValue> args);

private:
  friend class Go;

  explicit BindingValue(Value value);

  Value ToValue();

  Value value_;
};

class Go {
public:
  using Func = std::function<BindingValue(std::vector<BindingValue>)>;

  Go();
  int Run();
  int Run(int argc, char** argv);
  int Run(const std::vector<std::string>& args);
  void Bind(const std::string& name, Func func);

private:
  class Import : public IImport {
  public:
    explicit Import(Go* go);

{{range $value := .ImportFuncs}}{{$value.CppDecl "    " false true}}

{{end}}
  private:
    Go* go_;
  };

  class JSValues : public IObject {
  public:
    explicit JSValues(Go* go);
    Value Get(const std::string& key) override;
    void Set(const std::string& key, Value value) override;
    void Delete(const std::string& key) override;

  private:
    Go* go_;
  };

  class Bindings : public IObject {
  public:
    explicit Bindings(std::map<std::string, Func> funcs);
    Value Get(const std::string& key) override;
    void Set(const std::string& key, Value value) override;
    void Delete(const std::string& key) override;

    void Set(const std::string& key, Func func);

  private:
    std::map<std::string, Func> funcs_;
  };

  Value LoadValue(int32_t addr);
  void StoreValue(int32_t addr, Value v);
  std::vector<Value> LoadSliceOfValues(int32_t addr);
  void Exit(int32_t code);
  void Resume();
  Value MakeFuncWrapper(int32_t id);
  void DebugWrite(BytesSegment bytes);
  int64_t PreciseNowInNanoseconds();
  double UnixNowInMilliseconds();
  int32_t SetTimeout(double interval);
  void ClearTimeout(int32_t id);
  void GetRandomBytes(BytesSegment bytes);

  Import import_;
  Writer debug_writer_;
  // A TaskQueue must be destructed after the timers are destructed.
  TaskQueue task_queue_;

  Value pending_event_;
  std::map<int32_t, std::unique_ptr<Timer>> scheduled_timeouts_;
  int32_t next_callback_timeout_id_ = 1;

  std::unique_ptr<Inst> inst_;
  std::unique_ptr<Mem> mem_;
  std::map<int32_t, Value> values_;
  std::map<int32_t, double> go_ref_counts_;
  std::map<Value, int32_t> ids_;
  std::stack<int32_t> id_pool_;
  bool exited_ = false;
  int32_t exit_code_ = 0;

  std::chrono::high_resolution_clock::time_point start_time_point_ = std::chrono::high_resolution_clock::now();

  std::map<std::string, Func> bindings_;
};

}

#endif  // {{.IncludeGuard}}

`))

var goCppTmpl = template.Must(template.New("go.cpp").Parse(`// Code generated by go2cpp. DO NOT EDIT.

#include "{{.IncludePath}}go.h"

#include <cassert>
#include <cmath>
#include <iostream>
#include <limits>
#include <random>

namespace {{.Namespace}} {

namespace {

// TODO: Merge this with the same one in js.cpp.
void error(const std::string& msg) {
  std::cerr << msg << std::endl;
  assert(false);
  std::exit(1);
}

}

BindingValue::BindingValue() {
}

BindingValue::BindingValue(bool b)
    : value_{b} {
}

BindingValue::BindingValue(double num)
    : value_{num} {
}

BindingValue::BindingValue(const char* str)
    : value_{str} {
}

BindingValue::BindingValue(const std::string& str)
    : value_{str} {
}

BindingValue::BindingValue(const std::vector<uint8_t>& bytes)
    : value_{bytes} {
}

BindingValue::BindingValue(Value value)
    : value_{value} {
}

bool BindingValue::IsNull() const {
  return value_.IsNull();
}

bool BindingValue::IsUndefined() const {
  return value_.IsUndefined();
}

bool BindingValue::IsBool() const {
  return value_.IsBool();
}

bool BindingValue::IsNumber() const {
  return value_.IsNumber();
}

bool BindingValue::IsString() const {
  return value_.IsString();
}

bool BindingValue::IsBytes() const {
  return value_.IsBytes();
}

bool BindingValue::IsFunction() const {
  return value_.IsJSObject() && value_.ToJSObject().IsFunction();
}

bool BindingValue::ToBool() const {
  return value_.ToBool();
}

double BindingValue::ToNumber() const {
  return value_.ToNumber();
}

std::string BindingValue::ToString() const {
  return value_.ToString();
}

const std::vector<uint8_t>& BindingValue::ToBytes() const {
  return value_.ToBytes();
}

BindingValue BindingValue::Invoke() {
  return Invoke(std::vector<BindingValue>{});
}

BindingValue BindingValue::Invoke(std::vector<BindingValue> args) {
  std::vector<Value> objs(args.size());
  for (int i = 0; i < args.size(); i++) {
    objs[i] = args[i].ToValue();
  }
  return BindingValue{value_.ToJSObject().Invoke(Value{}, objs)};
}

Value BindingValue::ToValue() {
  return value_;
}

Go::Go()
    : import_{this},
      debug_writer_{std::cerr} {
}

int Go::Run() {
  return Run(std::vector<std::string>{});
}

int Go::Run(int argc, char** argv) {
  std::vector<std::string> args(argv, argv + argc);
  return Run(args);
}

int Go::Run(const std::vector<std::string>& args) {
  mem_ = std::make_unique<Mem>();
  inst_ = std::make_unique<Inst>(mem_.get(), &import_);

  std::shared_ptr<JSObject> global = JSObject::Global();
  std::unique_ptr<Bindings> bindings = std::make_unique<Bindings>(std::move(bindings_));
  global->Set("c++", Value{std::make_shared<JSObject>(std::move(bindings))});

  values_ = std::map<int32_t, Value>{
    {0, Value{std::nan("")}},
    {1, Value{0.0}},
    {2, Value{}},
    {3, Value{true}},
    {4, Value{false}},
    {5, Value{global}},
    {6, Value{JSObject::Go(std::make_unique<JSValues>(this))}},
  };
  static const double inf = std::numeric_limits<double>::infinity();
  go_ref_counts_ = std::map<int32_t, double>{
    {0, inf},
    {1, inf},
    {2, inf},
    {3, inf},
    {4, inf},
    {5, inf},
    {6, inf},
  };
  ids_ = std::map<Value, int32_t>{
    {values_[1], 1},
    {values_[2], 2},
    {values_[3], 3},
    {values_[4], 4},
    {values_[5], 5},
    {values_[6], 6},
  };

  id_pool_ = std::stack<int32_t>();
  exited_ = false;
  exit_code_ = 0;

  int32_t offset = 4096;
  auto str_ptr = [this, &offset](const std::string& str) -> int32_t {
    int32_t ptr = offset;
    std::vector<uint8_t> bytes(str.begin(), str.end());
    bytes.push_back('\0');
    mem_->StoreBytes(offset, bytes);
    offset += bytes.size();
    if (offset % 8 != 0) {
      offset += 8 - (offset % 8);
    }
    return ptr;
  };

  // 'js' is requried as the first argument.
  std::vector<std::string> margs = args;
  if (margs.size() == 0) {
    margs.push_back("js");
  } else {
    margs[0] = "js";
  }
  int argc = margs.size();
  std::vector<int32_t> argv_ptrs;
  for (const std::string& arg : margs) {
    argv_ptrs.push_back(str_ptr(arg));
  }
  argv_ptrs.push_back(0);
  // TODO: Add environment variables.
  argv_ptrs.push_back(0);

  int32_t argv = offset;
  for (int32_t ptr : argv_ptrs) {
    mem_->StoreInt32(offset, ptr);
    mem_->StoreInt32(offset + 4, 0);
    offset += 8;
  }

  inst_->run(argc, argv);

  while (!exited_) {
    TaskQueue::Task task = task_queue_.Dequeue();
    if (task) {
      task();
    }
  }

  return static_cast<int>(exit_code_);
}

void Go::Bind(const std::string& name, Func func) {
  bindings_[name] = std::move(func);
}

Go::Import::Import(Go* go)
    : go_{go} {
}

{{range $value := .ImportFuncs}}{{$value.CppImpl "Go::Import" ""}}
{{end}}

Go::JSValues::JSValues(Go* go)
    : go_(go) {
}

Value Go::JSValues::Get(const std::string& key) {
  if (key == "_makeFuncWrapper") {
    return Value{std::make_shared<JSObject>(
      [this](Value self, std::vector<Value> args) -> Value {
        return go_->MakeFuncWrapper(static_cast<int32_t>(args[0].ToNumber()));
      }
    )};
  }
  if (key == "_pendingEvent") {
    return go_->pending_event_;
  }
  error("Go::JSValues::Get: key not found: " + key);
  return Value{};
}

void Go::JSValues::Set(const std::string& key, Value value) {
  if (key == "_pendingEvent") {
    go_->pending_event_ = value;
    return;
  }
  error("key not found: " + key);
}

void Go::JSValues::Delete(const std::string& key) {
  error("Go::JSValues::Delete: not implemented");
}

Go::Bindings::Bindings(std::map<std::string, Func> funcs)
    : funcs_{std::move(funcs)} {
}

Value Go::Bindings::Get(const std::string& key) {
  if (funcs_.find(key) == funcs_.end()) {
    error("Go::Bindings::Get: " + key + " not found");
    return Value{};
  }
  Func& fn = funcs_[key];
  std::shared_ptr<JSObject> jsobj = std::make_shared<JSObject>(
    [&fn](Value self, std::vector<Value> args) -> Value {
      std::vector<BindingValue> goargs(args.size());
      for (int i = 0; i < goargs.size(); i++) {
        goargs[i] = BindingValue{args[i]};
      }
      BindingValue result = fn(goargs);
      return result.ToValue();
    });
  return Value{jsobj};
}

void Go::Bindings::Set(const std::string& key, Value value) {
  error("Go::Bindings::Set: not implemented");
}

void Go::Bindings::Delete(const std::string& key) {
  error("Go::Bindings::Delete: not implemented");
}

void Go::Bindings::Set(const std::string& key, Func func) {
  funcs_[key] = func;
}

Value Go::LoadValue(int32_t addr) {
  double f = mem_->LoadFloat64(addr);
  if (f == 0) {
    return Value::Undefined();
  }
  if (!std::isnan(f)) {
    return Value{f};
  }
  int32_t id = static_cast<int32_t>(mem_->LoadUint32(addr));
  return values_[id];
}

void Go::StoreValue(int32_t addr, Value v) {
  static const int32_t kNaNHead = 0x7FF80000;

  if (v.IsNumber() && v.ToNumber() != 0.0) {
    double n = v.ToNumber();
    if (std::isnan(n)) {
      mem_->StoreInt32(addr + 4, kNaNHead);
      mem_->StoreInt32(addr, 0);
      return;
    }
    mem_->StoreFloat64(addr, n);
    return;
  }

  if (v.IsUndefined()) {
    mem_->StoreFloat64(addr, 0);
    return;
  }

  int32_t id = 0;
  auto it = ids_.find(v);
  if (it != ids_.end()) {
    id = it->second;
  } else {
    if (id_pool_.size()) {
      id = id_pool_.top();
      id_pool_.pop();
    } else {
      id = values_.size();
    }
    values_[id] = v;
    go_ref_counts_[id] = 0;
    ids_[v] = id;
  }
  go_ref_counts_[id]++;

  int32_t type_flag = 0;
  if (v.IsString()) {
    type_flag = 2;
    // There is no counterpart for Symbol in C++, then type_flag = 3 is not used.
  } else if (v.IsJSObject() && v.ToJSObject().IsFunction()) {
    type_flag = 4;
  } else if (!v.IsNull() && !v.IsNumber() && !v.IsBool()) {
    type_flag = 1;
  }
  mem_->StoreInt32(addr + 4, kNaNHead | type_flag);
  mem_->StoreInt32(addr, id);
}

std::vector<Value> Go::LoadSliceOfValues(int32_t addr) {
  int32_t array = static_cast<int32_t>(mem_->LoadInt64(addr));
  int32_t len = static_cast<int32_t>(mem_->LoadInt64(addr + 8));
  std::vector<Value> a(len);
  for (int32_t i = 0; i < len; i++) {
    a[i] = LoadValue(array + i * 8);
  }
  return a;
}

void Go::Exit(int32_t code) {
  exit_code_ = code;
}

void Go::Resume() {
  if (exited_) {
    error("Go program has already exited");
  }
  inst_->resume();
  // Post a null task and procceed the loop.
  task_queue_.Enqueue(TaskQueue::Task{});
}

Value Go::MakeFuncWrapper(int32_t id) {
  return Value{std::make_shared<JSObject>(
    [this, id](Value self, std::vector<Value> args) -> Value {
      auto evt = Value{std::make_shared<JSObject>(std::map<std::string, Value>{
        {"id", Value{static_cast<double>(id)}},
        {"this", self},
        {"args", Value{args}},
      })};
      pending_event_ = evt;
      Resume();
      return JSObject::ReflectGet(evt, "result");
    }
  )};
}

void Go::DebugWrite(BytesSegment bytes) {
  debug_writer_.Write(std::vector<uint8_t>(bytes.begin(), bytes.end()));
}

int64_t Go::PreciseNowInNanoseconds() {
  std::chrono::nanoseconds duration = std::chrono::high_resolution_clock::now() - start_time_point_;
  return duration.count();
}

double Go::UnixNowInMilliseconds() {
  std::chrono::milliseconds now =
      std::chrono::duration_cast<std::chrono::milliseconds>(std::chrono::system_clock::now().time_since_epoch());
  return now.count();
}

int32_t Go::SetTimeout(double interval) {
  int32_t id = next_callback_timeout_id_;
  next_callback_timeout_id_++;
  std::unique_ptr<Timer> timer = std::make_unique<Timer>(
    [this, id] {
      task_queue_.Enqueue([this, id]{
        Resume();
        while (scheduled_timeouts_.find(id) != scheduled_timeouts_.end()) {
          // for some reason Go failed to register the timeout event, log and try again
          // (temporary workaround for https://github.com/golang/go/issues/28975)
          Resume();
        }
      });
    }, interval);
  scheduled_timeouts_[id] = std::move(timer);
  return id;
}

void Go::ClearTimeout(int32_t id) {
  if (scheduled_timeouts_.find(id) != scheduled_timeouts_.end()) {
    scheduled_timeouts_[id]->Stop();
  }
  scheduled_timeouts_.erase(id);
}

void Go::GetRandomBytes(BytesSegment bytes) {
  // TODO: Use cryptographically strong random values instead of std::random_device.
  static std::random_device rd;
  std::uniform_int_distribution<uint8_t> dist(0, 255);
  for (int i = 0; i < bytes.size(); i++) {
    bytes[i] = dist(rd);
  }
}

}
`))
