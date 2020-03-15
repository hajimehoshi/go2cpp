// SPDX-License-Identifier: Apache-2.0

// +build example

package main

import (
	"syscall/js"
)

type game struct {
	counter int
}

func (g *game) update() {
	g.counter += 2
	if g.counter >= 256 {
		g.counter = 0
	}
}

func (g *game) draw() int {
	return g.counter
}

func main() {
	g := &game{}

	updatef := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		g.update()
		return nil
	})
	defer updatef.Release()

	drawf := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		return g.draw()
	})
	defer drawf.Release()

	js.Global().Get(".net").Get("Go2DotNet.Example.MonoGame.GoGameRunner").Call("Run", updatef, drawf);
}
