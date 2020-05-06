// SPDX-License-Identifier: Apache-2.0

// +build example

package main

import (
	"syscall/js"
)

func main() {
	i := 0
	f := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		println("Hi", i)
		i++
		return nil
	})
	defer f.Release()

	// For the definition of CallTwice, see main.cpp.
	js.Global().Get("c++").Call("CallTwice", f)
}
