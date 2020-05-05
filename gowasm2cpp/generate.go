// SPDX-License-Identifier: Apache-2.0

package gowasm2cpp

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
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
		var idx int
		for _, e := range f.Wasm.Body.Locals {
			for i := 0; i < int(e.Count); i++ {
				locals = append(locals, fmt.Sprintf("%s local%d = 0;", wasmTypeToReturnType(e.Type).Cpp(), idx+len(f.Wasm.Sig.ParamTypes)))
				idx++
			}
		}
		var err error
		body, err = f.bodyToCpp()
		if err != nil {
			return "", err
		}
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

func Generate(outDir string, wasmFile string, namespace string) error {
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
				Namespace    string
				ImportFuncs  []*wasmFunc
			}{
				IncludeGuard: includeGuard(namespace) + "_GO_H",
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
				Namespace   string
				ImportFuncs []*wasmFunc
			}{
				Namespace:   namespace,
				ImportFuncs: ifs,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	g.Go(func() error {
		return writeBits(outDir, namespace)
	})
	g.Go(func() error {
		return writeJS(outDir, namespace)
	})
	g.Go(func() error {
		return writeInst(outDir, namespace, ifs, fs, exports, globals, types, tables)
	})
	g.Go(func() error {
		return writeMem(outDir, namespace, int(mod.Memory.Entries[0].Limits.Initial), data)
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
#include <condition_variable>
#include <functional>
#include <map>
#include <memory>
#include <mutex>
#include <queue>
#include <stack>
#include <string>
#include <thread>
#include <vector>
#include "autogen/js.h"
#include "autogen/inst.h"
#include "autogen/mem.h"

namespace {{.Namespace}} {

class Mem;

class TaskQueue {
public:
  using Task = std::function<void()>;

  void Enqueue(Task task);
  Task Dequeue();

private:
  std::mutex mutex_;
  std::condition_variable cond_;
  std::queue<Task> queue_;
};

class Timer {
public:
  Timer(std::function<void()> func, double interval);
  ~Timer();

  void Stop();

private:
  enum class Result {
    Timeout,
    NoTimeout,
  };

  Result WaitFor(double milliseconds);

  // A mutex and a condition variable must be constructed before the thread starts.
  std::mutex mutex_;
  std::condition_variable cond_;
  std::thread thread_;
  bool stopped_ = false;
};

class Go {
public:
  Go();
  int Run();
  int Run(int argc, char** argv);
  int Run(const std::vector<std::string>& args);

private:
  class Import : public IImport {
  public:
    explicit Import(Go* go);

{{range $value := .ImportFuncs}}{{$value.CppDecl "    " false true}}

{{end}}
  private:
    Go* go_;
  };

  class JSValues : public JSObject::IValues {
  public:
    explicit JSValues(Go* go);
    Object Get(const std::string& key) override;
    void Set(const std::string& key, Object value) override;
    void Remove(const std::string& key) override;

  private:
    Go* go_;
  };

  Object LoadValue(int32_t addr);
  void StoreValue(int32_t addr, Object v);
  std::vector<Object> LoadSliceOfValues(int32_t addr);
  void Exit(int32_t code);
  void Resume();
  Object MakeFuncWrapper(int32_t id);
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

  Object pending_event_;
  std::map<int32_t, std::unique_ptr<Timer>> scheduled_timeouts_;
  int32_t next_callback_timeout_id_ = 1;

  std::unique_ptr<Inst> inst_;
  std::unique_ptr<Mem> mem_;
  std::map<int32_t, Object> values_;
  std::map<int32_t, int32_t> go_ref_counts_;
  std::map<Object, int32_t> ids_;
  std::stack<int32_t> id_pool_;
  bool exited_ = false;
  int32_t exit_code_ = 0;

  std::chrono::high_resolution_clock::time_point start_time_point_ = std::chrono::high_resolution_clock::now();
};

}

#endif  // {{.IncludeGuard}}

`))

var goCppTmpl = template.Must(template.New("go.cpp").Parse(`// Code generated by go2cpp. DO NOT EDIT.

#include "autogen/go.h"

#include <cmath>
#include <iostream>
#include <random>

namespace {{.Namespace}} {

namespace {

// TODO: Merge this with the same one in js.cpp.
void error(const std::string& msg) {
  std::cerr << msg << std::endl;
  std::exit(1);
}

}

void TaskQueue::Enqueue(Task task) {
  std::lock_guard<std::mutex> lock{mutex_};
  queue_.push(task);
  cond_.notify_one();
}

TaskQueue::Task TaskQueue::Dequeue() {
  std::unique_lock<std::mutex> lock{mutex_};
  cond_.wait(lock, [this]{ return !queue_.empty(); });
  Task task = queue_.front();
  queue_.pop();
  return task;
}

Timer::Timer(std::function<void()> func, double interval)
    : thread_{[this, interval](std::function<void()> func) {
        Result result = WaitFor(interval);
        if (result == Timer::Result::NoTimeout) {
          return;
        }
        func();
      }, std::move(func)} {
}

Timer::~Timer() {
  if (thread_.joinable()) {
    thread_.join();
  }
}

void Timer::Stop() {
  std::lock_guard<std::mutex> lock{mutex_};
  stopped_ = true;
  cond_.notify_one();
}

Timer::Result Timer::WaitFor(double milliseconds) {
  std::unique_lock<std::mutex> lock{mutex_};
  auto duration = std::chrono::duration<double, std::milli>(milliseconds);
  bool result = cond_.wait_for(lock, duration, [this]{return stopped_;});
  return result ? Result::NoTimeout : Result::Timeout;
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
  values_ = std::map<int32_t, Object>{
    {0, Object{std::nan("")}},
    {1, Object{0.0}},
    {2, Object{}},
    {3, Object{true}},
    {4, Object{false}},
    {5, JSObject::Global()},
    {6, Object{JSObject::Go(std::make_unique<JSValues>(this))}},
  };
  go_ref_counts_.clear();
  ids_.clear();
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

Go::Import::Import(Go* go)
    : go_{go} {
}

{{range $value := .ImportFuncs}}{{$value.CppImpl "Go::Import" ""}}
{{end}}

Go::JSValues::JSValues(Go* go)
    : go_(go) {
}

Object Go::JSValues::Get(const std::string& key) {
  if (key == "_makeFuncWrapper") {
    return Object{std::make_shared<JSObject>(
      [this](Object self, std::vector<Object> args) -> Object {
        return go_->MakeFuncWrapper(static_cast<int32_t>(args[0].ToNumber()));
      }
    )};
  }
  if (key == "_pendingEvent") {
    return go_->pending_event_;
  }
  error("key not found: " + key);
  return Object{};
}

void Go::JSValues::Set(const std::string& key, Object value) {
  if (key == "_pendingEvent") {
    go_->pending_event_ = value;
    return;
  }
  error("key not found: " + key);
}

void Go::JSValues::Remove(const std::string& key) {
  error("not implemented: JSValues::Remove");
}

Object Go::LoadValue(int32_t addr) {
  double f = mem_->LoadFloat64(addr);
  if (f == 0) {
    return Object::Undefined();
  }
  if (!std::isnan(f)) {
    return Object{f};
  }
  int32_t id = static_cast<int32_t>(mem_->LoadUint32(addr));
  return values_[id];
}

void Go::StoreValue(int32_t addr, Object v) {
  static const int32_t kNaNHead = 0x7FF80000;

  if (v.IsNumber()) {
    double n = v.ToNumber();
    if (std::isnan(n)) {
      mem_->StoreInt32(addr + 4, kNaNHead);
      mem_->StoreInt32(addr, 0);
      return;
    }
    if (n == 0) {
      mem_->StoreInt32(addr + 4, kNaNHead);
      mem_->StoreInt32(addr, 1);
      return;
    }
    mem_->StoreFloat64(addr, n);
    return;
  }

  if (v.IsUndefined()) {
    mem_->StoreFloat64(addr, 0);
    return;
  }

  if (v.IsNull()) {
    mem_->StoreInt32(addr + 4, kNaNHead);
    mem_->StoreInt32(addr, 2);
    return;
  }

  if (v.IsBool()) {
    mem_->StoreInt32(addr + 4, kNaNHead);
    if (v.ToBool()) {
      mem_->StoreInt32(addr, 3);
    } else {
      mem_->StoreInt32(addr, 4);
    }
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

  int32_t type_flag = 1;
  if (v.IsString()) {
    type_flag = 2;
    // There is no counterpart for Symbol in C++, then type_flag = 3 is not used.
  } else if (v.IsJSObject() && v.ToJSObject()->IsFunction()) {
    type_flag = 4;
  }
  mem_->StoreInt32(addr + 4, kNaNHead | type_flag);
  mem_->StoreInt32(addr, id);
}

std::vector<Object> Go::LoadSliceOfValues(int32_t addr) {
  int32_t array = static_cast<int32_t>(mem_->LoadInt64(addr));
  int32_t len = static_cast<int32_t>(mem_->LoadInt64(addr + 8));
  std::vector<Object> a(len);
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

Object Go::MakeFuncWrapper(int32_t id) {
  return Object{std::make_shared<JSObject>(
    [this, id](Object self, std::vector<Object> args) -> Object {
      auto evt = Object{std::make_shared<JSObject>(std::map<std::string, Object>{
        {"id", Object{static_cast<double>(id)}},
        {"this", self},
        {"args", Object{args}},
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
