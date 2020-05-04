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
		body = []string{"  throw new NotImplementedException();"}
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

func (t *wasmType) CSharp(indent string) (string, error) {
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
		args = append(args, fmt.Sprintf("%s arg%d", wasmTypeToReturnType(t).CSharp(), i))
	}

	return fmt.Sprintf("%sprivate delegate %s Type%d(%s);", indent, retType.CSharp(), t.Index, strings.Join(args, ", ")), nil
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

#include <cstdint>
#include <chrono>
#include <map>
#include <memory>
#include <stack>
#include <vector>
#include "autogen/js.h"
#include "autogen/inst.h"
#include "autogen/mem.h"

namespace {{.Namespace}} {

class Mem;

class Go {
public:
  Go();

private:
  class Import : public IImport {
  public:
    explicit Import(Go* go);

{{range $value := .ImportFuncs}}{{$value.CppDecl "    " false true}}

{{end}}
  private:
    Go* go_;
  };

  Object LoadValue(int32_t addr);
  void StoreValue(int32_t addr, Object v);
  std::vector<Object> LoadSliceOfValues(int32_t addr);
  void Exit(int32_t code);
  void DebugWrite(BytesSegment bytes);
  int64_t PreciseNowInNanoseconds();
  double UnixNowInMilliseconds();
  int32_t SetTimeout(double interval);
  void ClearTimeout(int32_t id);
  void GetRandomBytes(BytesSegment bytes);

  Import import_;
  Writer debug_writer_;

  // JSObject pending_event_;
  // Dictionary<int, Timer> scheduled_timeouts_ = new Dictionary<int, Timer>();
  // int next_callback_timeout_id_ = 1;

  std::unique_ptr<Inst> inst_;
  std::unique_ptr<Mem> mem_;
  std::map<int32_t, Object> values_;
  std::map<int32_t, int32_t> go_ref_counts_;
  std::map<Object, int32_t> ids_;
  std::stack<int32_t> id_pool_;
  bool exited_ = false;

  // BlockingCollection<Action> taskQueue;

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
#include <string>

namespace {{.Namespace}} {

namespace {

// TODO: Merge this with the same one in js.cpp.
void error(const std::string& msg) {
  std::cerr << msg << std::endl;
  std::exit(1);
}

}

Go::Go()
    : import_{this},
      debug_writer_{std::cerr} {
}

Go::Import::Import(Go* go)
    : go_{go} {
}

{{range $value := .ImportFuncs}}{{$value.CppImpl "Go::Import" ""}}
{{end}}

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
  std::vector<Object> a;
  for (int32_t i = 0; i < len; i++) {
    a[i] = LoadValue(array + i * 8);
  }
  return a;
}

void Go::Exit(int32_t code) {
  if (code) {
    std::cerr << "exit code: " << code << std::endl;
  }
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
  /*
            var id = this.nextCallbackTimeoutId;
            this.nextCallbackTimeoutId++;

            Timer timer = new Timer(interval);
            timer.Elapsed += (sender, e) => {
                this.taskQueue.Add(() => {
                    this.Resume();
                    while (this.scheduledTimeouts.ContainsKey(id))
                    {
                        // for some reason Go failed to register the timeout event, log and try again
                        // (temporary workaround for https://github.com/golang/go/issues/28975)
                        this.Resume();
                    }
                });
            };
            timer.AutoReset = false;
            timer.Start();

            this.scheduledTimeouts[id] = timer;

            return id;*/
  return 0;
}

void Go::ClearTimeout(int32_t id) {
  /*          if (this.scheduledTimeouts.ContainsKey(id))
            {
                this.scheduledTimeouts[id].Stop();
            }
            this.scheduledTimeouts.Remove(id);
        }*/
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

/*using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Diagnostics;
using System.Security.Cryptography;
using System.Text;
using System.Timers;

namespace {{.Namespace}}
{
    public class Go
    {
        private class JSValues : JSObject.IValues
        {
            public JSValues(Go go)
            {
                this.go = go;
            }

            public object Get(string key)
            {
                switch (key)
                {
                case "_makeFuncWrapper":
                    return new JSObject((object self, object[] args) =>
                        {
                            return this.go.MakeFuncWrapper((int)JSObject.ToDouble(args[0]));
                        });
                case "_pendingEvent":
                    return this.go.pendingEvent;
                }
                throw new KeyNotFoundException(key);
            }

            public void Set(string key, object value)
            {
                switch (key)
                {
                case "_pendingEvent":
                    this.go.pendingEvent = (JSObject)value;
                    return;
                }
                throw new KeyNotFoundException(key);
            }

            public void Remove(string key)
            {
                throw new NotImplementedException();
            }

            private Go go;
        }

        public Go()
        {
            this.taskQueue = new BlockingCollection<Action>(new ConcurrentQueue<Action>());
        }

        internal object LoadValue(int addr)
        internal void StoreValue(int addr, object v)
        internal object[] LoadSliceOfValues(int addr)

        public void Run()
        {
            Run(new string[0]);
        }

        public void Run(string[] args)
        {
            this.debugWriter = new Writer(Console.Error);
            this.stopwatch = Stopwatch.StartNew();
            this.mem = new Mem();
            this.inst = new Inst(this.mem, this.import);
            this.values = new Dictionary<int, object>
            {
                {0, double.NaN},
                {1, 0.0},
                {2, null},
                {3, true},
                {4, false},
                {5, JSObject.Global},
                {6, JSObject.Go(new JSValues(this))},
            };
            this.goRefCounts = new Dictionary<int, int>();
            this.ids = new Dictionary<object, int>();
            this.idPool = new Stack<int>();
            this.exited = false;

            int offset = 4096;
            Func<string, int> strPtr = (string str) => {
                int ptr = offset;
                byte[] bytes = Encoding.UTF8.GetBytes(str + '\0');
                this.mem.StoreBytes(offset, bytes);
                offset += bytes.Length;
                if (offset % 8 != 0)
                {
                    offset += 8 - (offset % 8);
                }
                return ptr;
            };

            // 'js' is requried as the first argument.
            if (args.Length == 0)
            {
                args = new string[] { "js" };
            }
            else
            {
                args[0] = "js";
            }
            int argc = args.Length;
            List<int> argvPtrs = new List<int>();
            foreach (string arg in args)
            {
                argvPtrs.Add(strPtr(arg));
            }
            argvPtrs.Add(0);
            // TODO: Add environment variables.
            argvPtrs.Add(0);

            int argv = offset;
            foreach (int ptr in argvPtrs)
            {
                this.mem.StoreInt32(offset, ptr);
                this.mem.StoreInt32(offset + 4, 0);
                offset += 8;
            }

            this.inst.run(argc, argv);
            for (;;)
            {
                if (this.exited)
                {
                    return;
                }
                var task = this.taskQueue.Take();
                if (task != null)
                {
                    task();
                }
            }
        }

        private void Exit(int code)

        private void Resume()
        {
            if (this.exited)
            {
                throw new Exception("Go program has already exited");
            }
            this.inst.resume();
            // Post a null task and procceed the loop.
            this.taskQueue.Add(null);
        }

        private JSObject MakeFuncWrapper(int id)
        {
            return new JSObject((object self, object[] args) =>
            {
                var evt = new JSObject(new Dictionary<string, object>()
                {
                    {"id", id},
                    {"this", self},
                    {"args", args ?? new object[0]},
                });
                this.pendingEvent = evt;
                this.Resume();
                return JSObject.ReflectGet(evt, "result");
            });
        }

        private void DebugWrite(IEnumerable<byte> bytes)
        private long PreciseNowInNanoseconds()
        private double UnixNowInMilliseconds()
        private int SetTimeout(double interval)
        private void ClearTimeout(int id)
        private void GetRandomBytes(BytesSegment bytes)

        private static long nanosecPerTick = (1_000_000_000L) / Stopwatch.Frequency;

        private Import import;

        private Writer debugWriter;
        private Stopwatch stopwatch;

        private JSObject pendingEvent;
        private Dictionary<int, Timer> scheduledTimeouts = new Dictionary<int, Timer>();
        private int nextCallbackTimeoutId = 1;
        private Inst inst;
        private Mem mem;
        private Dictionary<int, object> values;
        private Dictionary<int, int> goRefCounts;
        private Dictionary<object, int> ids;
        private Stack<int> idPool;
        private bool exited;
        private RNGCryptoServiceProvider rngCsp = new RNGCryptoServiceProvider();

        private BlockingCollection<Action> taskQueue;
    }
}*/
`))
