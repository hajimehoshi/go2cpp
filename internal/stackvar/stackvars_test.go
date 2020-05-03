// SPDX-License-Identifier: Apache-2.0

package stackvar_test

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/hajimehoshi/go2cpp/internal/stackvar"
)

func TestPushPop(t *testing.T) {
	s := StackVars{
		VarName: func(idx int) string {
			return fmt.Sprintf("stack%d", idx)
		},
	}
	s.Push("foo", I32)
	s.Push("bar", I64)
	{
		e, ty := s.Pop()
		if got, want := e, "bar"; got != want {
			t.Errorf("got: %v, want: %v", got, want)
		}
		if got, want := ty, I64; got != want {
			t.Errorf("got: %v, want: %v", got, want)
		}
	}
	{
		e, ty := s.Pop()
		if got, want := e, "foo"; got != want {
			t.Errorf("got: %v, want: %v", got, want)
		}
		if got, want := ty, I32; got != want {
			t.Errorf("got: %v, want: %v", got, want)
		}
	}
}

func TestPeep(t *testing.T) {
	s := StackVars{
		VarName: func(idx int) string {
			return fmt.Sprintf("stack%d", idx)
		},
	}
	s.Push("foo", F32)
	s.Push("bar", F64)

	ls, v := s.Peep()
	if got, want := strings.Join(ls, "\n"), "double stack0 = (bar);"; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
	if got, want := v, "stack0"; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}

	ls, v = s.Peep()
	if got, want := strings.Join(ls, "\n"), ""; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
	if got, want := v, "stack0"; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}

	{
		e, ty := s.Pop()
		if got, want := e, "stack0"; got != want {
			t.Errorf("got: %v, want: %v", got, want)
		}
		if got, want := ty, F64; got != want {
			t.Errorf("got: %v, want: %v", got, want)
		}
	}

	ls, v = s.Peep()
	if got, want := strings.Join(ls, "\n"), "float stack1 = (foo);"; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
	if got, want := v, "stack1"; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}

	ls, v = s.Peep()
	if got, want := strings.Join(ls, "\n"), ""; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
	if got, want := v, "stack1"; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}
