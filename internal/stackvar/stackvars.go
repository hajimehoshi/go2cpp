// SPDX-License-Identifier: Apache-2.0

package stackvar

import (
	"fmt"
)

type StackVars struct {
	VarName func(idx int) string

	exprs  []string
	idx    int
	peeped bool
}

func (s *StackVars) PushLhs() string {
	n := s.VarName(s.idx)
	s.Push(n)
	s.idx++
	return n
}

func (s *StackVars) Push(expr string) {
	s.peeped = false
	s.exprs = append(s.exprs, expr)
}

func (s *StackVars) Pop() string {
	s.peeped = false
	l := s.exprs[len(s.exprs)-1]
	s.exprs = s.exprs[:len(s.exprs)-1]
	return l
}

func (s *StackVars) Peep() ([]string, string) {
	if s.peeped {
		return nil, s.exprs[len(s.exprs)-1]
	}

	l := s.Pop()
	n := s.PushLhs()
	s.peeped = true
	return []string{fmt.Sprintf("var %s = (%s);", n, l)}, n
}

func (s *StackVars) Empty() bool {
	return len(s.exprs) == 0
}
