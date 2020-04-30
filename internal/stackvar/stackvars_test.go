// SPDX-License-Identifier: Apache-2.0

package stackvar_test

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/hajimehoshi/go2dotnet/internal/stackvar"
)

func TestPushPop(t *testing.T) {
	s := StackVars{
		VarName: func(idx int) string {
			return fmt.Sprintf("stack%d", idx)
		},
	}
	s.Push("foo")
	s.Push("bar")
	if got, want := s.Pop(), "bar"; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
	if got, want := s.Pop(), "foo"; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestPeep(t *testing.T) {
	s := StackVars{
		VarName: func(idx int) string {
			return fmt.Sprintf("stack%d", idx)
		},
	}
	s.Push("foo")
	s.Push("bar")

	ls, v := s.Peep()
	if got, want := strings.Join(ls, "\n"), "var stack0 = (bar);"; got != want {
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

	if got, want := s.Pop(), "stack0"; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}

	ls, v = s.Peep()
	if got, want := strings.Join(ls, "\n"), "var stack1 = (foo);"; got != want {
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
