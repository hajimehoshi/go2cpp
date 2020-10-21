// SPDX-License-Identifier: Apache-2.0

// +build example

package main

import (
	"github.com/hajimehoshi/go2cpp/binding"
)

func main() {
	i := 0
	f := binding.FuncOf(func(args []binding.Value) interface{} {
		println("Hi", i)
		i++
		return nil
	})
	defer f.Release()

	// For the definition of CallTwice, see main.cpp.
	binding.Call("CallTwice", f)
}
