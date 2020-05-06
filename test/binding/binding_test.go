// SPDX-License-Identifier: Apache-2.0

package binding_test

import (
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
