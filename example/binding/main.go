// SPDX-License-Identifier: Apache-2.0

package main

import (
	"syscall/js"
)

func main() {
	ext := js.Global().Get("c#").Get("Go2DotNet.Example.Binding.External")
	println(ext.Get("StaticField").Int())
	println(ext.Get("StaticProperty").Int())
	ext.Call("StaticMethod", "Hello from Go")
	ext.Get("StaticMethod").Invoke("Hello from Go (Invoke)")
}
