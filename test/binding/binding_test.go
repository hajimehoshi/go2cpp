// SPDX-License-Identifier: Apache-2.0

package main

import (
	"syscall/js"
	"testing"
)

func TestStatic(t *testing.T) {
	cls := js.Global().Get(".net").Get("Go2DotNet.Test.Binding.Testing")

	cls.Set("StaticField", 1)
	cls.Set("StaticProperty", 2)
	if got, want := cls.Get("StaticField").Int(), 1; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
	if got, want := cls.Get("StaticProperty").Int(), 2; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}

	if got, want := cls.Call("StaticMethod", "Hello from Go").String(), "Hello from Go and C#"; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestInstance(t *testing.T) {
	cls := js.Global().Get(".net").Get("Go2DotNet.Test.Binding.Testing")
	inst := cls.New("foo", 3)

	inst.Set("InstanceField", 4)
	inst.Set("InstanceProperty", 5)
	if got, want := inst.Get("InstanceField").Int(), 4; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
	if got, want := inst.Get("InstanceProperty").Int(), 5; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}

	if got, want := inst.Call("InstanceMethod", "bar").String(), "str: foo, num: 3, arg: bar"; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestFunc(t *testing.T) {
	cls := js.Global().Get(".net").Get("Go2DotNet.Test.Binding.Testing")
	inst := cls.New("", 0)

	f := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		return args[0].String() + "+go"
	})
	defer f.Release()
	if got, want := inst.Call("InvokeGo", f, "arg").String(), "arg+go"; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}
