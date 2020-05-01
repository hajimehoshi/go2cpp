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
	Type    *wasmType
	Wasm    wasm.Function
	Index   int
	Import  bool
	BodyStr string
}

func (f *wasmFunc) Identifier() string {
	return identifierFromString(f.Wasm.Name)
}

var funcTmpl = template.Must(template.New("func").Parse(`// OriginalName: {{.OriginalName}}
// Index:        {{.Index}}
{{if .WithBody}}{{if .Public}}public{{else}}private{{end}} {{end}}{{.ReturnType}} {{.Name}}({{.Args}}){{if .WithBody}}
{
{{range .Locals}}    {{.}}
{{end}}{{if .Locals}}
{{end}}{{range .Body}}{{.}}
{{end}}}{{else}};{{end}}`))

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

func (f *wasmFunc) CSharp(indent string, public bool, withBody bool) (string, error) {
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
		args = append(args, fmt.Sprintf("%s local%d", wasmTypeToReturnType(t).CSharp(), i))
	}

	var locals []string
	var body []string
	if withBody {
		if f.BodyStr != "" {
			body = strings.Split(f.BodyStr, "\n")
		} else if f.Wasm.Body != nil {
			var idx int
			for _, e := range f.Wasm.Body.Locals {
				for i := 0; i < int(e.Count); i++ {
					locals = append(locals, fmt.Sprintf("%s local%d = 0;", wasmTypeToReturnType(e.Type).CSharp(), idx+len(f.Wasm.Sig.ParamTypes)))
					idx++
				}
			}
			var err error
			body, err = f.bodyToCSharp()
			if err != nil {
				return "", err
			}
		} else {
			body = []string{"    throw new NotImplementedException();"}
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
		Public       bool
		WithBody     bool
	}{
		OriginalName: f.Wasm.Name,
		Name:         identifierFromString(f.Wasm.Name),
		Index:        f.Index,
		ReturnType:   retType.CSharp(),
		Args:         strings.Join(args, ", "),
		Locals:       locals,
		Body:         body,
		Public:       public,
		WithBody:     withBody,
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

func (e *wasmExport) CSharp(indent string) (string, error) {
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
		args = append(args, fmt.Sprintf("%s arg%d", wasmTypeToReturnType(t).CSharp(), i))
		argsToPass = append(argsToPass, fmt.Sprintf("arg%d", i))
	}

	str := fmt.Sprintf(`public %s %s(%s)
{
    %s%s(%s);
}
`, retType.CSharp(), e.Name, strings.Join(args, ", "), ret, identifierFromString(f.Wasm.Name), strings.Join(argsToPass, ", "))

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

func (g *wasmGlobal) CSharp(indent string) string {
	return fmt.Sprintf("%sprivate %s global%d = %d;", indent, wasmTypeToReturnType(g.Type).CSharp(), g.Index, g.Init)
}

type wasmType struct {
	Sig   *wasm.FunctionSig
	Index int
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

	var ifs []*wasmFunc
	for i, e := range mod.Import.Entries {
		name := e.FieldName
		ifs = append(ifs, &wasmFunc{
			Type: types[e.Type.(wasm.FuncImport).Type],
			Wasm: wasm.Function{
				Sig:  types[e.Type.(wasm.FuncImport).Type].Sig,
				Name: name,
			},
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
			Index: i + len(mod.Import.Entries),
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
		out, err := os.Create(filepath.Join(outDir, "Go.cs"))
		if err != nil {
			return err
		}
		defer out.Close()

		if err := goTmpl.Execute(out, struct {
			Namespace   string
			ImportFuncs []*wasmFunc
		}{
			Namespace:   namespace,
			ImportFuncs: ifs,
		}); err != nil {
			return err
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
		return writeInstCS(outDir, namespace, ifs, fs, exports, globals, types, tables)
	})
	g.Go(func() error {
		return writeMemInitData(outDir, data)
	})
	g.Go(func() error {
		return writeMemCS(outDir, namespace, int(mod.Memory.Entries[0].Limits.Initial))
	})

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

var goTmpl = template.Must(template.New("Go.cs").Parse(`// Code generated by go2dotnet. DO NOT EDIT.

using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Diagnostics;
using System.Security.Cryptography;
using System.Text;
using System.Timers;

namespace {{.Namespace}}
{
    internal interface IImport
    {
{{- range $value := .ImportFuncs}}
{{$value.CSharp "        " false false}}{{end}}
    }

    public class Go
    {
        class Import : IImport
        {
            internal Import(Go go)
            {
                this.go = go;
            }
{{range $value := .ImportFuncs}}
{{$value.CSharp "            " true true}}{{end}}
            private Go go;
        }

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
            this.import = new Import(this);
            this.taskQueue = new BlockingCollection<Action>(new ConcurrentQueue<Action>());
        }

        internal object LoadValue(int addr)
        {
            double f = this.mem.LoadFloat64(addr);
            if (f == 0)
            {
                return JSObject.Undefined;
            }
            if (!double.IsNaN(f))
            {
                return f;
            }
            int id = (int)this.mem.LoadUint32(addr);
            return this.values[id];
        }

        internal void StoreValue(int addr, object v)
        {
            const int NaNHead = 0x7FF80000;
            double? d = JSObject.ToDouble(v);
            if (d.HasValue)
            {
                if (double.IsNaN(d.Value))
                {
                    this.mem.StoreInt32(addr + 4, NaNHead);
                    this.mem.StoreInt32(addr, 0);
                    return;
                }
                if (d.Value == 0)
                {
                    this.mem.StoreInt32(addr + 4, NaNHead);
                    this.mem.StoreInt32(addr, 1);
                    return;
                }
                this.mem.StoreFloat64(addr, d.Value);
                return;
            }
            if (v == JSObject.Undefined)
            {
                this.mem.StoreFloat64(addr, 0);
                return;
            }
            switch (v)
            {
            case null:
                this.mem.StoreInt32(addr + 4, NaNHead);
                this.mem.StoreInt32(addr, 2);
                return;
            case true:
                this.mem.StoreInt32(addr + 4, NaNHead);
                this.mem.StoreInt32(addr, 3);
                return;
            case false:
                this.mem.StoreInt32(addr + 4, NaNHead);
                this.mem.StoreInt32(addr, 4);
                return;
            }
            int id = 0;
            if (this.ids.ContainsKey(v))
            {
                id = this.ids[v];
            }
            else
            {
                if (this.idPool.Count > 0)
                {
                    id = this.idPool.Pop();
                }
                else
                {
                    id = this.values.Count;
                }
                this.values[id] = v;
                this.goRefCounts[id] = 0;
                this.ids[v] = id;
            }
            this.goRefCounts[id]++;
            int typeFlag = 1;
            if (v is string)
            {
                typeFlag = 2;
                // There is no counterpart for Symbol in C#, then typeFlag = 3 is not used.
            }
            else if (v is JSObject && ((JSObject)v).IsFunction)
            {
                typeFlag = 4;
            }
            this.mem.StoreInt32(addr + 4, NaNHead | typeFlag);
            this.mem.StoreInt32(addr, id);
        }

        internal object[] LoadSliceOfValues(int addr)
        {
            var array = (int)this.mem.LoadInt64(addr);
            var len = (int)this.mem.LoadInt64(addr + 8);
            var a = new object[len];
            for (int i = 0; i < len; i++)
            {
                a[i] = this.LoadValue(array + i * 8);
            }
            return a;
        }

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
        {
            if (code != 0)
            {
                Console.Error.WriteLine($"exit code: {code}");
            }
        }

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
        {
            this.debugWriter.Write(bytes);
        }

        private long PreciseNowInNanoseconds()
        {
            return this.stopwatch.ElapsedTicks * nanosecPerTick;
        }

        private double UnixNowInMilliseconds()
        {
            return (DateTime.UtcNow.Subtract(new DateTime(1970, 1, 1))).TotalMilliseconds;
        }

        private int SetTimeout(double interval)
        {
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

            return id;
        }

        private void ClearTimeout(int id)
        {
            if (this.scheduledTimeouts.ContainsKey(id))
            {
                this.scheduledTimeouts[id].Stop();
            }
            this.scheduledTimeouts.Remove(id);
        }

        private byte[] GetRandomBytes(int length)
        {
            var bytes = new byte[length];
            this.rngCsp.GetBytes(bytes);
            return bytes;
        }

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
}
`))
