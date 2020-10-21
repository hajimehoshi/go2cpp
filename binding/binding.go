// SPDX-License-Identifier: Apache-2.0

package binding

import (
	"syscall/js"
)

type Value = js.Value

func Call(name string, args ...interface{}) Value {
	jsargs := make([]interface{}, len(args))
	for i, arg := range args {
		switch arg := arg.(type) {
		case Func:
			jsargs[i] = arg.f
		default:
			jsargs[i] = arg
		}
	}
	return js.Global().Get("c++").Call(name, jsargs...)
}

type Func struct {
	f js.Func
}

func FuncOf(f func(args []Value) interface{}) Func {
	jsf := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		return f(args)
	})
	return Func{f: jsf}
}

func (f Func) Release() {
	f.f.Release()
}
