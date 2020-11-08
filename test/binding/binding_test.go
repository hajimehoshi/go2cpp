// SPDX-License-Identifier: Apache-2.0

package binding_test

import (
	"fmt"
	"syscall/js"
	"testing"
)

func TestIdentity(t *testing.T) {
	if got, want := js.Global().Get("c++").Call("Identity", true).Bool(), true; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
	if got, want := js.Global().Get("c++").Call("Identity", 42).Int(), 42; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
	if got, want := js.Global().Get("c++").Call("Identity", 3.14159).Float(), 3.14159; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
	if got, want := js.Global().Get("c++").Call("Identity", "foo").String(), "foo"; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}

	if got := js.Global().Get("c++").Call("Identity", nil); !got.Equal(js.Null()) {
		t.Errorf("got: %v, want: js.Null()", got)
	}
	if got := js.Global().Get("c++").Call("Identity", js.Undefined()); !got.Equal(js.Undefined()) {
		t.Errorf("got: %v, want: js.Undefined()", got)
	}

	// It is OK to pass an object. BindingValue doesn't offer a way to manipulte the object.
	if got := js.Global().Get("c++").Call("Identity", js.Global()); !got.Equal(js.Global()) {
		t.Errorf("got: %v, want: js.Undefined()", got)
	}
}

func TestInvoke(t *testing.T) {
	f := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		return fmt.Sprintf("%t, %d, %s", args[0].Bool(), args[1].Int(), args[2].String())
	})
	defer f.Release()

	got := js.Global().Get("c++").Call("Invoke", f, true, 2, "third arg").String()
	want := "true, 2, third arg"
	if got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestSum(t *testing.T) {
	if got, want := js.Global().Get("c++").Call("Sum", 1, 2, 3, 4, 5).Int(), 15; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestBool(t *testing.T) {
	if got, want := js.Global().Get("c++").Call("Bool", true).Bool(), true; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
	if got, want := js.Global().Get("c++").Call("Bool", false).Bool(), false; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestJSBindingWindow(t *testing.T) {
	js.Global().Set("foo", 1)
	if got, want := js.Global().Get("foo").Int(), 1; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
	if got, want := js.Global().Get("window").Get("foo").Int(), 1; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestJSBindingObject(t *testing.T) {
	js.Global().Set("foo", js.Global().Get("Object").New())
	js.Global().Get("foo").Set("bar", 2)
	if got, want := js.Global().Get("foo").Get("bar").Int(), 2; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestJSBindingFunc(t *testing.T) {
	f := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		return 3
	})
	defer f.Release()

	js.Global().Set("foo", js.Global().Get("Object").New())
	js.Global().Get("foo").Set("bar", f)
	if got, want := js.Global().Get("foo").Get("bar").Invoke().Int(), 3; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}
