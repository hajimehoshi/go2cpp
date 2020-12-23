// SPDX-License-Identifier: Apache-2.0

package stackvar

import (
	"fmt"
	"strings"
)

type Type int

const (
	I32 Type = iota
	I64
	F32
	F64
)

func (t Type) Cpp() string {
	switch t {
	case I32:
		return "int32_t"
	case I64:
		return "int64_t"
	case F32:
		return "float"
	case F64:
		return "double"
	default:
		panic("not reached")
	}
}

type StackVars struct {
	VarName func(idx int) string

	exprs  []string
	types  []Type
	idx    int
	peeped bool
}

func (s *StackVars) PushLhs(t Type) string {
	n := s.VarName(s.idx)
	s.Push(n, t)
	s.idx++
	return n
}

func (s *StackVars) Push(expr string, t Type) {
	s.peeped = false
	s.exprs = append(s.exprs, expr)
	s.types = append(s.types, t)
}

func (s *StackVars) Pop() (string, Type) {
	s.peeped = false
	l := s.exprs[len(s.exprs)-1]
	t := s.types[len(s.types)-1]
	s.exprs = s.exprs[:len(s.exprs)-1]
	s.types = s.types[:len(s.types)-1]
	return l, t
}

func (s *StackVars) Peep() ([]string, string) {
	if s.peeped {
		return nil, s.exprs[len(s.exprs)-1]
	}

	l, t := s.Pop()
	n := s.PushLhs(t)
	s.peeped = true
	return []string{fmt.Sprintf("%s %s = (%s);", t.Cpp(), n, l)}, n
}

func (s *StackVars) Len() int {
	return len(s.exprs)
}

func (s *StackVars) Empty() bool {
	return len(s.exprs) == 0
}

// IncludesInNonTop reports whether str is included in the exprs except for the top expr.
func (s *StackVars) IncludesInNonTop(str string) bool {
	for _, expr := range s.exprs[:len(s.exprs)-1] {
		if strings.Contains(expr, str) {
			return true
		}
	}
	return false
}
