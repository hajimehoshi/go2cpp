// SPDX-License-Identifier: Apache-2.0

// +build example

package main

import (
	"syscall/js"
)

func main() {
	t := js.Global().Get(".net").Get("Go2DotNet.Example.Binding.Test")
	f := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		println("Hi")
		return nil
	})
	defer f.Release()
	t.Call("CallTwice", f)
}
