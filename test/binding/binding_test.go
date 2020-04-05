// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
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

func TestFuncNoArgs(t *testing.T) {
	cls := js.Global().Get(".net").Get("Go2DotNet.Test.Binding.Testing")
	inst := cls.New("", 0)

	a := 0
	f := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		a = 1
		return nil
	})
	defer f.Release()
	inst.Call("InvokeGoWithoutArgs", f)
	if got, want := a, 1; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestFuncReturningZero(t *testing.T) {
	cls := js.Global().Get(".net").Get("Go2DotNet.Test.Binding.Testing")
	inst := cls.New("", 0)

	f := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		return 0
	})
	defer f.Release()
	if got, want := inst.Call("InvokeGoAndReturnDouble", f).Int(), 0; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestCopyBytes(t *testing.T) {
	cls := js.Global().Get(".net").Get("Go2DotNet.Test.Binding.Testing")
	inst := cls.New("", 0)

	bs := []byte{1, 2, 3}
	arr := js.Global().Get("Uint8Array").New(len(bs))
	if got, want := js.CopyBytesToJS(arr, bs), 3; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
	arr2 := inst.Call("DoubleBytes", arr)
	bs2 := make([]byte, arr2.Length())
	if got, want := js.CopyBytesToGo(bs2, arr2), 3; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
	if got, want := bs2, []byte{2, 4, 6}; !bytes.Equal(got, want) {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestReturnAndPassInstance(t *testing.T) {
	cls := js.Global().Get(".net").Get("Go2DotNet.Test.Binding.Testing")
	if got, want := cls.Call("StaticMethodToReturnInstance", "foo", 1).Call("InstanceMethod", "bar").String(), "str: foo, num: 1, arg: bar"; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}

	inst := cls.New("foo", 1)
	inst2 := inst.Call("Clone")

	if got, want := inst2.Call("InstanceMethod", "bar").String(), "str: foo, num: 1, arg: bar"; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}

	inst3 := cls.New("bar", 2)
	inst3.Call("CopyFrom", inst)
	if got, want := inst3.Call("InstanceMethod", "bar").String(), "str: foo, num: 1, arg: bar"; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}

	// TODO: Add tests fields and properties to treat instances.
}
