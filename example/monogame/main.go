// SPDX-License-Identifier: Apache-2.0

// +build example

package main

import (
	"syscall/js"
)

func update() {
	println("update")
}

func draw() {
	println("draw")
}

func main() {
	updatef := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		update()
		return nil
	})
	defer updatef.Release()
	drawf := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		draw()
		return nil
	})
	defer drawf.Release()
	js.Global().Get(".net").Get("Go2DotNet.Example.MonoGame.GoGameRunner").Call("Run", updatef, drawf);
}
