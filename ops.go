// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/go-interpreter/wagon/disasm"
	"github.com/go-interpreter/wagon/wasm"
	"github.com/go-interpreter/wagon/wasm/operators"
)

type ReturnType int

const (
	ReturnTypeVoid ReturnType = iota
	ReturnTypeI32
	ReturnTypeI64
	ReturnTypeF32
	ReturnTypeF64
)

func (r ReturnType) CSharp() string {
	switch r {
	case ReturnTypeVoid:
		return "void"
	case ReturnTypeI32:
		return "int"
	case ReturnTypeI64:
		return "long"
	case ReturnTypeF32:
		return "float"
	case ReturnTypeF64:
		return "double"
	default:
		panic("not reached")
	}
}

type Stack struct {
	newIdx int
	stack  []int
}

func (s *Stack) Push() int {
	idx := s.newIdx
	s.stack = append(s.stack, idx)
	s.newIdx++
	return idx
}

func (s *Stack) Pop() int {
	idx := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
	return idx
}

func (s *Stack) Peep() int {
	return s.stack[len(s.stack)-1]
}

func (s *Stack) PeepLevel(level int) int {
	return s.stack[len(s.stack)-1-level]
}

func (s *Stack) Len() int {
	return len(s.stack)
}

type BlockType int

const (
	BlockTypeBlock BlockType = iota
	BlockTypeLoop
	BlockTypeIf
)

type BlockStack struct {
	types []BlockType
	s     Stack
}

func (b *BlockStack) Push(btype BlockType) int {
	b.types = append(b.types, btype)
	return b.s.Push()
}

func (b *BlockStack) Pop() (int, BlockType) {
	btype := b.types[len(b.types)-1]
	b.types = b.types[:len(b.types)-1]
	return b.s.Pop(), btype
}

func (b *BlockStack) Peep() (int, BlockType) {
	return b.s.Peep(), b.types[len(b.types)-1]
}

func (b *BlockStack) PeepLevel(level int) (int, BlockType) {
	return b.s.PeepLevel(level), b.types[len(b.types)-1-level]
}

func (b *BlockStack) Len() int {
	return b.s.Len()
}

func (b *BlockStack) IndentLevel() int {
	l := 0
	for _, t := range b.types {
		if t == BlockTypeIf {
			l++
		}
	}
	return l
}

func (f *Func) bodyToCSharp() ([]string, error) {
	sig := f.Wasm.Sig
	funcs := f.Funcs
	types := f.Types

	dis, err := disasm.NewDisassembly(f.Wasm, f.Mod)
	if err != nil {
		return nil, err
	}

	var body []string
	idxStack := &Stack{}
	blockStack := &BlockStack{}
	var tmpidx int

	appendBody := func(str string, args ...interface{}) {
		str = fmt.Sprintf(str, args...)
		if str == "{" && len(body) > 0 && strings.HasSuffix(body[len(body)-1], "{") {
			body[len(body)-1] += "{"
			return
		}

		level := blockStack.IndentLevel() + 1
		if strings.HasSuffix(str, ":;") {
			level--
		}
		indent := strings.Repeat("    ", level)
		body = append(body, indent+str)
	}

	for _, instr := range dis.Code {
		switch instr.Op.Code {
		case operators.Unreachable:
			appendBody(`Debug.Assert(false, "not reached");`)
		case operators.Nop:
			// Do nothing
		case operators.Block:
			if instr.Immediates[0] != wasm.BlockTypeEmpty {
				return nil, fmt.Errorf("'block' taking types is not implemented")
			}
			appendBody("{")
			blockStack.Push(BlockTypeBlock)
		case operators.Loop:
			if instr.Immediates[0] != wasm.BlockTypeEmpty {
				return nil, fmt.Errorf("'loop' taking types is not implemented")
			}
			appendBody("{")
			l := blockStack.Push(BlockTypeLoop)
			appendBody("label%d:;", l)
		case operators.If:
			if instr.Immediates[0] != wasm.BlockTypeEmpty {
				return nil, fmt.Errorf("'if' taking types is not implemented")
			}
			idx := idxStack.Pop()
			appendBody("if (stack%d != 0)", idx)
			appendBody("{")
			blockStack.Push(BlockTypeIf)
		case operators.Else:
			appendBody("}")
			appendBody("else")
			appendBody("{")
		case operators.End:
			idx, btype := blockStack.Pop()
			appendBody("}")
			if btype != BlockTypeLoop {
				appendBody("label%d:;", idx)
			}
		case operators.Br:
			level := instr.Immediates[0].(uint32)
			l, _ := blockStack.PeepLevel(int(level))
			appendBody("goto label%d;", l)
		case operators.BrIf:
			level := instr.Immediates[0].(uint32)
			l, _ := blockStack.PeepLevel(int(level))
			idx := idxStack.Pop()
			appendBody("if (stack%d != 0) goto label%d;", idx, l)
		case operators.BrTable:
			idx := idxStack.Pop()
			appendBody("switch (stack%d)", idx)
			appendBody("{")
			len := int(instr.Immediates[0].(uint32))
			for i := 0; i < len; i++ {
				level := int(instr.Immediates[1+i].(uint32))
				l, _ := blockStack.PeepLevel(level)
				appendBody("case %d: goto label%d;", i, l)
			}
			l, _ := blockStack.PeepLevel(int(instr.Immediates[len].(uint32)))
			appendBody("default: goto label%d;", l)
			appendBody("}")
		case operators.Return:
			switch len(sig.ReturnTypes) {
			case 0:
				appendBody("return;")
			default:
				appendBody("return stack%d;", idxStack.Peep())
			}

		case operators.Call:
			f := funcs[instr.Immediates[0].(uint32)]

			args := make([]string, len(f.Wasm.Sig.ParamTypes))
			for i := range f.Wasm.Sig.ParamTypes {
				args[len(f.Wasm.Sig.ParamTypes)-i-1] = fmt.Sprintf("stack%d", idxStack.Pop())
			}

			var ret string
			if len(f.Wasm.Sig.ReturnTypes) > 0 {
				ret = fmt.Sprintf("var stack%d = ", idxStack.Push())
			}

			var imp string
			if f.Import {
				imp = "import_."
			}
			appendBody("%s%s%s(%s);", ret, imp, identifierFromString(f.Wasm.Name), strings.Join(args, ", "))
		case operators.CallIndirect:
			idx := idxStack.Pop()
			typeid := instr.Immediates[0].(uint32)
			t := types[typeid]

			args := make([]string, len(t.Sig.ParamTypes))
			for i := range t.Sig.ParamTypes {
				args[len(t.Sig.ParamTypes)-i-1] = fmt.Sprintf("stack%d", idxStack.Pop())
			}

			var ret string
			if len(t.Sig.ReturnTypes) > 0 {
				ret = fmt.Sprintf("var stack%d = ", idxStack.Push())
			}

			appendBody("%s((Type%d)(funcs_[table_[0][stack%d]]))(%s);", ret, typeid, idx, strings.Join(args, ", "))

		case operators.Drop:
			idxStack.Pop()
		case operators.Select:
			cond := idxStack.Pop()
			arg1 := idxStack.Pop()
			arg0 := idxStack.Peep()
			appendBody("stack%[2]d = (stack%[1]d != 0) ? stack%[2]d : stack%[3]d;", cond, arg0, arg1)

		case operators.GetLocal:
			idx := idxStack.Push()
			appendBody("var stack%d = local%d;", idx, instr.Immediates[0])
		case operators.SetLocal:
			idx := idxStack.Pop()
			appendBody("local%d = stack%d;", instr.Immediates[0], idx)
		case operators.TeeLocal:
			idx := idxStack.Peep()
			appendBody("local%d = stack%d;", instr.Immediates[0], idx)
		case operators.GetGlobal:
			idx := idxStack.Push()
			appendBody("var stack%d = global%d;", idx, instr.Immediates[0])
		case operators.SetGlobal:
			idx := idxStack.Pop()
			appendBody("global%d = stack%d;", instr.Immediates[0], idx)

		case operators.I32Load:
			// TODO: Implement this.
			idxStack.Pop()
			idx := idxStack.Push()
			appendBody("int stack%d = 0 /* TODO */;", idx)
		case operators.I64Load:
			// TODO: Implement this.
			idxStack.Pop()
			idx := idxStack.Push()
			appendBody("long stack%d = 0 /* TODO */;", idx)
		case operators.F32Load:
			// TODO: Implement this.
			idxStack.Pop()
			idx := idxStack.Push()
			appendBody("float stack%d = 0 /* TODO */;", idx)
		case operators.F64Load:
			// TODO: Implement this.
			idxStack.Pop()
			idx := idxStack.Push()
			appendBody("double stack%d = 0 /* TODO */;", idx)
		case operators.I32Load8s:
			// TODO: Implement this.
			idxStack.Pop()
			idx := idxStack.Push()
			appendBody("int stack%d = 0 /* TODO */;", idx)
		case operators.I32Load8u:
			// TODO: Implement this.
			idxStack.Pop()
			idx := idxStack.Push()
			appendBody("int stack%d = 0 /* TODO */;", idx)
		case operators.I32Load16s:
			// TODO: Implement this.
			idxStack.Pop()
			idx := idxStack.Push()
			appendBody("int stack%d = 0 /* TODO */;", idx)
		case operators.I32Load16u:
			// TODO: Implement this.
			idxStack.Pop()
			idx := idxStack.Push()
			appendBody("int stack%d = 0 /* TODO */;", idx)
		case operators.I64Load8s:
			// TODO: Implement this.
			idxStack.Pop()
			idx := idxStack.Push()
			appendBody("long stack%d = 0 /* TODO */;", idx)
		case operators.I64Load8u:
			// TODO: Implement this.
			idxStack.Pop()
			idx := idxStack.Push()
			appendBody("long stack%d = 0 /* TODO */;", idx)
		case operators.I64Load16s:
			// TODO: Implement this.
			idxStack.Pop()
			idx := idxStack.Push()
			appendBody("long stack%d = 0 /* TODO */;", idx)
		case operators.I64Load16u:
			// TODO: Implement this.
			idxStack.Pop()
			idx := idxStack.Push()
			appendBody("long stack%d = 0 /* TODO */;", idx)
		case operators.I64Load32s:
			// TODO: Implement this.
			idxStack.Pop()
			idx := idxStack.Push()
			appendBody("long stack%d = 0 /* TODO */;", idx)
		case operators.I64Load32u:
			// TODO: Implement this.
			idxStack.Pop()
			idx := idxStack.Push()
			appendBody("long stack%d = 0 /* TODO */;", idx)

		case operators.I32Store:
			// TODO: Implement this.
			idxStack.Pop()
			idxStack.Pop()
		case operators.I64Store:
			// TODO: Implement this.
			idxStack.Pop()
			idxStack.Pop()
		case operators.F32Store:
			// TODO: Implement this.
			idxStack.Pop()
			idxStack.Pop()
		case operators.F64Store:
			// TODO: Implement this.
			idxStack.Pop()
			idxStack.Pop()
		case operators.I32Store8:
			// TODO: Implement this.
			idxStack.Pop()
			idxStack.Pop()
		case operators.I32Store16:
			// TODO: Implement this.
			idxStack.Pop()
			idxStack.Pop()
		case operators.I64Store8:
			// TODO: Implement this.
			idxStack.Pop()
			idxStack.Pop()
		case operators.I64Store16:
			// TODO: Implement this.
			idxStack.Pop()
			idxStack.Pop()
		case operators.I64Store32:
			// TODO: Implement this.
			idxStack.Pop()
			idxStack.Pop()

		case operators.CurrentMemory:
			idx := idxStack.Push()
			// TOOD: Implement this.
			_ = idx
		case operators.GrowMemory:
			// TOOD: Implement this.

		case operators.I32Const:
			idx := idxStack.Push()
			appendBody("int stack%d = %d;", idx, instr.Immediates[0])
		case operators.I64Const:
			idx := idxStack.Push()
			appendBody("long stack%d = %d;", idx, instr.Immediates[0])
		case operators.F32Const:
			idx := idxStack.Push()
			if v := instr.Immediates[0].(float32); v == 0 {
				appendBody("float stack%d = 0;", idx)
			} else {
				bits := math.Float32bits(v)
				appendBody("uint tmp%d = %d; // %f", tmpidx, bits, v)
				appendBody("float stack%d = Unsafe.As<uint, float>(ref tmp%d);", idx, tmpidx)
				tmpidx++
			}
		case operators.F64Const:
			idx := idxStack.Push()
			if v := instr.Immediates[0].(float64); v == 0 {
				appendBody("double stack%d = 0;", idx)
			} else {
				bits := math.Float64bits(v)
				appendBody("ulong tmp%d = %d; // %f", tmpidx, bits, v)
				appendBody("double stack%d = Unsafe.As<ulong, double>(ref tmp%d);", idx, tmpidx)
				tmpidx++
			}

		case operators.I32Eqz:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d == 0) ? 1 : 0;", dst, arg)
		case operators.I32Eq:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d == stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32Ne:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d != stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32LtS:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d < stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32LtU:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = ((uint)stack%d < (uint)stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32GtS:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d > stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32GtU:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = ((uint)stack%d > (uint)stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32LeS:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d <= stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32LeU:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = ((uint)stack%d <= (uint)stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32GeS:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d >= stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32GeU:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = ((uint)stack%d >= (uint)stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64Eqz:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d == 0) ? 1 : 0;", dst, arg)
		case operators.I64Eq:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d == stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64Ne:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d != stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64LtS:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d < stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64LtU:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = ((ulong)stack%d < (ulong)stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64GtS:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d > stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64GtU:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = ((ulong)stack%d > (ulong)stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64LeS:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d <= stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64LeU:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = ((ulong)stack%d <= (ulong)stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64GeS:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d >= stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64GeU:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = ((ulong)stack%d >= (ulong)stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.F32Eq:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d == stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.F32Ne:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d != stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.F32Lt:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d < stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.F32Gt:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d > stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.F32Le:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d <= stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.F32Ge:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d >= stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.F64Eq:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d == stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.F64Ne:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d != stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.F64Lt:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d < stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.F64Gt:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d > stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.F64Le:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d <= stack%d) ? 1 : 0;", dst, arg0, arg1)
		case operators.F64Ge:
			arg1 := idxStack.Pop()
			arg0 := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (stack%d >= stack%d) ? 1 : 0;", dst, arg0, arg1)

		case operators.I32Clz:
			return nil, fmt.Errorf("I32Clz is not implemented")
		case operators.I32Ctz:
			return nil, fmt.Errorf("I32Ctz is not implemented")
		case operators.I32Popcnt:
			return nil, fmt.Errorf("I32Popcnt is not implemented")
		case operators.I32Add:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d += stack%d;", dst, arg)
		case operators.I32Sub:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d -= stack%d;", dst, arg)
		case operators.I32Mul:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d *= stack%d;", dst, arg)
		case operators.I32DivS:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d /= stack%d;", dst, arg)
		case operators.I32DivU:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%[1]d = (int)((uint)stack%[1]d / (uint)stack%[2]d);", dst, arg)
		case operators.I32RemS:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d %%= stack%d;", dst, arg)
		case operators.I32RemU:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%[1]d = (int)((uint)stack%[1]d %% (uint)stack%[2]d);", dst, arg)
		case operators.I32And:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d &= stack%d;", dst, arg)
		case operators.I32Or:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d |= stack%d;", dst, arg)
		case operators.I32Xor:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d ^= stack%d;", dst, arg)
		case operators.I32Shl:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d <<= stack%d;", dst, arg)
		case operators.I32ShrS:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d >>= stack%d;", dst, arg)
		case operators.I32ShrU:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%[1]d = (int)((uint)stack%[1]d >> stack%[2]d);", dst, arg)
		case operators.I32Rotl:
			return nil, fmt.Errorf("I32Rotl is not implemented")
		case operators.I32Rotr:
			return nil, fmt.Errorf("I32Rotr is not implemented")
		case operators.I64Clz:
			return nil, fmt.Errorf("I64Clz is not implemented")
		case operators.I64Ctz:
			return nil, fmt.Errorf("I64Ctz is not implemented")
		case operators.I64Popcnt:
			return nil, fmt.Errorf("I64Popcnt is not implemented")
		case operators.I64Add:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d += stack%d;", dst, arg)
		case operators.I64Sub:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d -= stack%d;", dst, arg)
		case operators.I64Mul:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d *= stack%d;", dst, arg)
		case operators.I64DivS:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d /= stack%d;", dst, arg)
		case operators.I64DivU:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%[1]d = (long)((ulong)stack%[1]d / (ulong)stack%[2]d);", dst, arg)
		case operators.I64RemS:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d %%= stack%d;", dst, arg)
		case operators.I64RemU:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%[1]d = (long)((ulong)stack%[1]d %% (ulong)stack%[2]d);", dst, arg)
		case operators.I64And:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d &= stack%d;", dst, arg)
		case operators.I64Or:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d |= stack%d;", dst, arg)
		case operators.I64Xor:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d ^= stack%d;", dst, arg)
		case operators.I64Shl:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d <<= (int)stack%d;", dst, arg)
		case operators.I64ShrS:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d >>= (int)stack%d;", dst, arg)
		case operators.I64ShrU:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%[1]d = (long)((ulong)stack%[1]d >> (int)stack%[2]d);", dst, arg)
		case operators.I64Rotl:
			return nil, fmt.Errorf("I64Rotl is not implemented")
		case operators.I64Rotr:
			return nil, fmt.Errorf("I64Rotr is not implemented")
		case operators.F32Abs:
			idx := idxStack.Peep()
			appendBody("stack%[1]d = Math.Abs(stack%[1]d);", idx)
		case operators.F32Neg:
			idx := idxStack.Peep()
			appendBody("stack%[1]d = -stack%[1]d;", idx)
		case operators.F32Ceil:
			idx := idxStack.Peep()
			appendBody("stack%[1]d = Math.Ceil(stack%[1]d);", idx)
		case operators.F32Floor:
			idx := idxStack.Peep()
			appendBody("stack%[1]d = Math.Floor(stack%[1]d);", idx)
		case operators.F32Trunc:
			idx := idxStack.Peep()
			appendBody("stack%[1]d = Math.Truncate(stack%[1]d);", idx)
		case operators.F32Nearest:
			return nil, fmt.Errorf("F32Nearest is not implemented yet")
		case operators.F32Sqrt:
			idx := idxStack.Peep()
			appendBody("stack%[1]d = Math.Sqrt(stack%[1]d);", idx)
		case operators.F32Add:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d += stack%d;", dst, arg)
		case operators.F32Sub:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d -= stack%d;", dst, arg)
		case operators.F32Mul:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d *= stack%d;", dst, arg)
		case operators.F32Div:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d /= stack%d;", dst, arg)
		case operators.F32Min:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%[1]d = Math.Min(stack%[1]d, stack%[2]d);", dst, arg)
		case operators.F32Max:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%[1]d = Math.Max(stack%[1]d, stack%[2]d);", dst, arg)
		case operators.F32Copysign:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%[1]d = Math.CopySign(stack%[1]d, stack%[2]d);", dst, arg)
		case operators.F64Abs:
			idx := idxStack.Peep()
			appendBody("stack%[1]d = Math.Abs(stack%[1]d);", idx)
		case operators.F64Neg:
			idx := idxStack.Peep()
			appendBody("stack%[1]d = -stack%[1]d;", idx)
		case operators.F64Ceil:
			idx := idxStack.Peep()
			appendBody("stack%[1]d = Math.Ceil(stack%[1]d);", idx)
		case operators.F64Floor:
			idx := idxStack.Peep()
			appendBody("stack%[1]d = Math.Floor(stack%[1]d);", idx)
		case operators.F64Trunc:
			idx := idxStack.Peep()
			appendBody("stack%[1]d = Math.Truncate(stack%[1]d);", idx)
		case operators.F64Nearest:
			return nil, fmt.Errorf("F64Nearest is not implemented yet")
		case operators.F64Sqrt:
			idx := idxStack.Peep()
			appendBody("stack%[1]d = Math.Sqrt(stack%[1]d);", idx)
		case operators.F64Add:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d += stack%d;", dst, arg)
		case operators.F64Sub:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d -= stack%d;", dst, arg)
		case operators.F64Mul:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d *= stack%d;", dst, arg)
		case operators.F64Div:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%d /= stack%d;", dst, arg)
		case operators.F64Min:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%[1]d = Math.Min(stack%[1]d, stack%[2]d);", dst, arg)
		case operators.F64Max:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%[1]d = Math.Max(stack%[1]d, stack%[2]d);", dst, arg)
		case operators.F64Copysign:
			arg := idxStack.Pop()
			dst := idxStack.Peep()
			appendBody("stack%[1]d = Math.CopySign(stack%[1]d, stack%[2]d);", dst, arg)

		case operators.I32WrapI64:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (int)stack%d;", dst, arg)
		case operators.I32TruncSF32:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (int)Math.Truncate(stack%d);", dst, arg)
		case operators.I32TruncUF32:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (int)((uint)Math.Truncate(stack%d));", dst, arg)
		case operators.I32TruncSF64:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (int)Math.Truncate(stack%d);", dst, arg)
		case operators.I32TruncUF64:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("int stack%d = (int)((uint)Math.Truncate(stack%d));", dst, arg)
		case operators.I64ExtendSI32:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("long stack%d = (long)stack%d;", dst, arg)
		case operators.I64ExtendUI32:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("long stack%d = (long)((ulong)stack%d);", dst, arg)
		case operators.I64TruncSF32:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("long stack%d = (long)Math.Truncate(stack%d);", dst, arg)
		case operators.I64TruncUF32:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("long stack%d = (long)((ulong)Math.Truncate(stack%d));", dst, arg)
		case operators.I64TruncSF64:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("long stack%d = (long)Math.Truncate(stack%d);", dst, arg)
		case operators.I64TruncUF64:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("long stack%d = (long)((ulong)Math.Truncate(stack%d));", dst, arg)
		case operators.F32ConvertSI32:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("float stack%d = (float)stack%d;", dst, arg)
		case operators.F32ConvertUI32:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("float stack%d = (float)((uint)stack%d);", dst, arg)
		case operators.F32ConvertSI64:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("float stack%d = (float)stack%d;", dst, arg)
		case operators.F32ConvertUI64:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("float stack%d = (float)((ulong)stack%d);", dst, arg)
		case operators.F32DemoteF64:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("float stack%d = (float)stack%d;", dst, arg)
		case operators.F64ConvertSI32:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("double stack%d = (double)stack%d;", dst, arg)
		case operators.F64ConvertUI32:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("double stack%d = (double)((uint)stack%d);", dst, arg)
		case operators.F64ConvertSI64:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("double stack%d = (double)stack%d;", dst, arg)
		case operators.F64ConvertUI64:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("double stack%d = (double)((long)stack%d);", dst, arg)
		case operators.F64PromoteF32:
			arg := idxStack.Pop()
			dst := idxStack.Push()
			appendBody("double stack%d = (double)stack%d;", dst, arg)

		case operators.I32ReinterpretF32:
			return nil, fmt.Errorf("I32ReinterpretF32 is not implemented yet")
		case operators.I64ReinterpretF64:
			return nil, fmt.Errorf("I64ReinterpretF64 is not implemented yet")
		case operators.F32ReinterpretI32:
			return nil, fmt.Errorf("F32ReinterpretI32 is not implemented yet")
		case operators.F64ReinterpretI64:
			return nil, fmt.Errorf("F64ReinterpretI64 is not implemented yet")

		default:
			return nil, fmt.Errorf("unexpected operator: %v", instr.Op)
		}
	}
	switch len(sig.ReturnTypes) {
	case 0:
		// Do nothing.
	case 1:
		if idxStack.Len() > 0 && dis.Code[len(dis.Code)-1].Op.Code != operators.Unreachable {
			idx := idxStack.Pop()
			appendBody(`return stack%d;`, idx)
		} else {
			// Throwing an exception might prevent optimization. Use assertion here.
			appendBody(`Debug.Assert(false, "not reached");`)
			appendBody(`return 0;`)
		}
	default:
		return nil, fmt.Errorf("unexpected num of return types: %d", len(sig.ReturnTypes))
	}

	return body, nil
}
