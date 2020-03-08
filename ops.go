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
	types     []BlockType
	rets      []string
	index     []*Stack
	s         Stack
	tmpindent int
}

func (b *BlockStack) UnindentTemporarily() {
	b.tmpindent--
}

func (b *BlockStack) IndentTemporarily() {
	b.tmpindent++
}

func (b *BlockStack) Push(btype BlockType, ret string) int {
	if b.index == nil {
		b.index = []*Stack{{}}
	}

	b.types = append(b.types, btype)
	b.rets = append(b.rets, ret)
	b.index = append(b.index, &Stack{})
	return b.s.Push()
}

func (b *BlockStack) Pop() (int, BlockType, string) {
	if b.index == nil {
		b.index = []*Stack{{}}
	}

	btype := b.types[len(b.types)-1]
	ret := b.rets[len(b.rets)-1]

	b.types = b.types[:len(b.types)-1]
	b.rets = b.rets[:len(b.rets)-1]
	b.index = b.index[:len(b.index)-1]
	return b.s.Pop(), btype, ret
}

func (b *BlockStack) Peep() (int, BlockType, string) {
	return b.s.Peep(), b.types[len(b.types)-1], b.rets[len(b.rets)-1]
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
	l += b.tmpindent
	return l
}

func (b *BlockStack) PushIndex() string {
	if b.index == nil {
		b.index = []*Stack{{}}
	}
	idx := b.index[len(b.index)-1].Push()
	if b.s.Len() > 0 {
		return fmt.Sprintf("%d_%d", b.s.Peep(), idx)
	}
	return fmt.Sprintf("%d", idx)
}

func (b *BlockStack) PopIndex() string {
	if b.index == nil {
		b.index = []*Stack{{}}
	}

	idx := b.index[len(b.index)-1].Pop()
	if b.s.Len() > 0 {
		return fmt.Sprintf("%d_%d", b.s.Peep(), idx)
	}
	return fmt.Sprintf("%d", idx)
}

func (b *BlockStack) PeepIndex() string {
	if b.index == nil {
		b.index = []*Stack{{}}
	}

	idx := b.index[len(b.index)-1].Peep()
	if b.s.Len() > 0 {
		return fmt.Sprintf("%d_%d", b.s.Peep(), idx)
	}
	return fmt.Sprintf("%d", idx)
}

func (b *BlockStack) HasIndex() bool {
	if len(b.index) == 0 {
		return false
	}
	return b.index[len(b.index)-1].Len() > 0
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
	blockStack := &BlockStack{}
	var tmpidx int

	appendBody := func(str string, args ...interface{}) {
		str = fmt.Sprintf(str, args...)
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
			var ret string
			if t := instr.Immediates[0]; t != wasm.BlockTypeEmpty {
				t := wasmTypeToReturnType(wasm.ValueType(t.(wasm.BlockType)))
				ret = blockStack.PushIndex()
				appendBody("%s stack%s;", t.CSharp(), ret)
			}
			blockStack.Push(BlockTypeBlock, ret)
		case operators.Loop:
			var ret string
			if t := instr.Immediates[0]; t != wasm.BlockTypeEmpty {
				t := wasmTypeToReturnType(wasm.ValueType(t.(wasm.BlockType)))
				ret = blockStack.PushIndex()
				appendBody("%s stack%s;", t.CSharp(), ret)
			}
			l := blockStack.Push(BlockTypeLoop, ret)
			appendBody("label%d:;", l)
		case operators.If:
			cond := blockStack.PopIndex()
			var ret string
			if t := instr.Immediates[0]; t != wasm.BlockTypeEmpty {
				t := wasmTypeToReturnType(wasm.ValueType(t.(wasm.BlockType)))
				ret = blockStack.PushIndex()
				appendBody("%s stack%s;", t.CSharp(), ret)
			}
			appendBody("if (stack%s != 0)", cond)
			appendBody("{")
			blockStack.Push(BlockTypeIf, ret)
		case operators.Else:
			if _, _, ret := blockStack.Peep(); ret != "" {
				idx := blockStack.PopIndex()
				appendBody("stack%s = stack%s", ret, idx)
			}
			blockStack.UnindentTemporarily()
			appendBody("}")
			appendBody("else")
			appendBody("{")
			blockStack.IndentTemporarily()
		case operators.End:
			if _, _, ret := blockStack.Peep(); ret != "" {
				idx := blockStack.PopIndex()
				appendBody("stack%s = stack%s", ret, idx)
			}
			idx, btype, _ := blockStack.Pop()
			if btype == BlockTypeIf {
				appendBody("}")
			}
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
			appendBody("if (stack%s != 0) goto label%d;", blockStack.PopIndex(), l)
		case operators.BrTable:
			appendBody("switch (stack%s)", blockStack.PopIndex())
			appendBody("{")
			len := int(instr.Immediates[0].(uint32))
			for i := 0; i < len; i++ {
				level := int(instr.Immediates[1+i].(uint32))
				l, _ := blockStack.PeepLevel(level)
				appendBody("case %d: goto label%d;", i, l)
			}
			l, _ := blockStack.PeepLevel(int(instr.Immediates[len+1].(uint32)))
			appendBody("default: goto label%d;", l)
			appendBody("}")
		case operators.Return:
			switch len(sig.ReturnTypes) {
			case 0:
				appendBody("return;")
			default:
				appendBody("return stack%s;", blockStack.PopIndex())
			}

		case operators.Call:
			f := funcs[instr.Immediates[0].(uint32)]

			args := make([]string, len(f.Wasm.Sig.ParamTypes))
			for i := range f.Wasm.Sig.ParamTypes {
				args[len(f.Wasm.Sig.ParamTypes)-i-1] = fmt.Sprintf("stack%s", blockStack.PopIndex())
			}

			var ret string
			if len(f.Wasm.Sig.ReturnTypes) > 0 {
				ret = fmt.Sprintf("var stack%s = ", blockStack.PushIndex())
			}

			var imp string
			if f.Import {
				imp = "import_."
			}
			appendBody("%s%s%s(%s);", ret, imp, identifierFromString(f.Wasm.Name), strings.Join(args, ", "))
		case operators.CallIndirect:
			idx := blockStack.PopIndex()
			typeid := instr.Immediates[0].(uint32)
			t := types[typeid]

			args := make([]string, len(t.Sig.ParamTypes))
			for i := range t.Sig.ParamTypes {
				args[len(t.Sig.ParamTypes)-i-1] = fmt.Sprintf("stack%s", blockStack.PopIndex())
			}

			var ret string
			if len(t.Sig.ReturnTypes) > 0 {
				ret = fmt.Sprintf("var stack%s = ", blockStack.PushIndex())
			}

			appendBody("%s((Type%d)(funcs_[table_[0][stack%s]]))(%s);", ret, typeid, idx, strings.Join(args, ", "))

		case operators.Drop:
			blockStack.PopIndex()
		case operators.Select:
			cond := blockStack.PopIndex()
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PeepIndex()
			appendBody("stack%[2]s = (stack%[1]s != 0) ? stack%[2]s : stack%[3]s;", cond, arg0, arg1)

		case operators.GetLocal:
			idx := blockStack.PushIndex()
			appendBody("var stack%s = local%d;", idx, instr.Immediates[0])
		case operators.SetLocal:
			idx := blockStack.PopIndex()
			appendBody("local%d = stack%s;", instr.Immediates[0], idx)
		case operators.TeeLocal:
			idx := blockStack.PeepIndex()
			appendBody("local%d = stack%s;", instr.Immediates[0], idx)
		case operators.GetGlobal:
			idx := blockStack.PushIndex()
			appendBody("var stack%s = global%d;", idx, instr.Immediates[0])
		case operators.SetGlobal:
			idx := blockStack.PopIndex()
			appendBody("global%d = stack%s;", instr.Immediates[0], idx)

		case operators.I32Load:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopIndex()
			idx := blockStack.PushIndex()
			appendBody("int stack%s = mem_.LoadInt32(stack%s + %d);", idx, addr, offset)
		case operators.I64Load:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopIndex()
			idx := blockStack.PushIndex()
			appendBody("long stack%s = mem_.LoadInt64(stack%s + %d);", idx, addr, offset)
		case operators.F32Load:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopIndex()
			idx := blockStack.PushIndex()
			appendBody("float stack%s = mem_.LoadFloat32(stack%s + %d);", idx, addr, offset)
		case operators.F64Load:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopIndex()
			idx := blockStack.PushIndex()
			appendBody("double stack%s = mem_.LoadFloat64(stack%s + %d);", idx, addr, offset)
		case operators.I32Load8s:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopIndex()
			idx := blockStack.PushIndex()
			appendBody("int stack%s = (int)mem_.LoadInt8(stack%s + %d);", idx, addr, offset)
		case operators.I32Load8u:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopIndex()
			idx := blockStack.PushIndex()
			appendBody("int stack%s = (int)mem_.LoadUint8(stack%s + %d);", idx, addr, offset)
		case operators.I32Load16s:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopIndex()
			idx := blockStack.PushIndex()
			appendBody("int stack%s = (int)mem_.LoadInt16(stack%s + %d);", idx, addr, offset)
		case operators.I32Load16u:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopIndex()
			idx := blockStack.PushIndex()
			appendBody("int stack%s = (int)mem_.LoadUint16(stack%s + %d);", idx, addr, offset)
		case operators.I64Load8s:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopIndex()
			idx := blockStack.PushIndex()
			appendBody("long stack%s = (long)mem_.LoadInt8(stack%s + %d);", idx, addr, offset)
		case operators.I64Load8u:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopIndex()
			idx := blockStack.PushIndex()
			appendBody("long stack%s = (long)mem_.LoadUint8(stack%s + %d);", idx, addr, offset)
		case operators.I64Load16s:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopIndex()
			idx := blockStack.PushIndex()
			appendBody("long stack%s = (long)mem_.LoadInt16(stack%s + %d);", idx, addr, offset)
		case operators.I64Load16u:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopIndex()
			idx := blockStack.PushIndex()
			appendBody("long stack%s = (long)mem_.LoadUint16(stack%s + %d);", idx, addr, offset)
		case operators.I64Load32s:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopIndex()
			idx := blockStack.PushIndex()
			appendBody("long stack%s = (long)mem_.LoadInt32(stack%s + %d);", idx, addr, offset)
		case operators.I64Load32u:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopIndex()
			idx := blockStack.PushIndex()
			appendBody("long stack%s = (long)mem_.LoadUint32(stack%s + %d);", idx, addr, offset)

		case operators.I32Store:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopIndex()
			addr := blockStack.PopIndex()
			appendBody("mem_.StoreInt32(stack%s + %d, stack%s);", addr, offset, idx)
		case operators.I64Store:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopIndex()
			addr := blockStack.PopIndex()
			appendBody("mem_.StoreInt64(stack%s + %d, stack%s);", addr, offset, idx)
		case operators.F32Store:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopIndex()
			addr := blockStack.PopIndex()
			appendBody("mem_.StoreFloat32(stack%s + %d, stack%s);", addr, offset, idx)
		case operators.F64Store:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopIndex()
			addr := blockStack.PopIndex()
			appendBody("mem_.StoreFloat64(stack%s + %d, stack%s);", addr, offset, idx)
		case operators.I32Store8:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopIndex()
			addr := blockStack.PopIndex()
			appendBody("mem_.StoreInt8(stack%s + %d, (sbyte)stack%s);", addr, offset, idx)
		case operators.I32Store16:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopIndex()
			addr := blockStack.PopIndex()
			appendBody("mem_.StoreInt16(stack%s + %d, (short)stack%s);", addr, offset, idx)
		case operators.I64Store8:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopIndex()
			addr := blockStack.PopIndex()
			appendBody("mem_.StoreInt8(stack%s + %d, (sbyte)stack%s);", addr, offset, idx)
		case operators.I64Store16:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopIndex()
			addr := blockStack.PopIndex()
			appendBody("mem_.StoreInt16(stack%s + %d, (short)stack%s);", addr, offset, idx)
		case operators.I64Store32:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopIndex()
			addr := blockStack.PopIndex()
			appendBody("mem_.StoreInt32(stack%s + %d, (int)stack%s);", addr, offset, idx)

		case operators.CurrentMemory:
			appendBody("int stack%s = mem_.Size;", blockStack.PushIndex())
		case operators.GrowMemory:
			delta := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = mem_.Grow(stack%s);", dst, delta)

		case operators.I32Const:
			idx := blockStack.PushIndex()
			appendBody("int stack%s = %d;", idx, instr.Immediates[0])
		case operators.I64Const:
			idx := blockStack.PushIndex()
			appendBody("long stack%s = %d;", idx, instr.Immediates[0])
		case operators.F32Const:
			idx := blockStack.PushIndex()
			if v := instr.Immediates[0].(float32); v == 0 {
				appendBody("float stack%s = 0;", idx)
			} else {
				bits := math.Float32bits(v)
				appendBody("uint tmp%d = %d; // %f", tmpidx, bits, v)
				appendBody("float stack%s = Unsafe.As<uint, float>(ref tmp%d);", idx, tmpidx)
				tmpidx++
			}
		case operators.F64Const:
			idx := blockStack.PushIndex()
			if v := instr.Immediates[0].(float64); v == 0 {
				appendBody("double stack%s = 0;", idx)
			} else {
				bits := math.Float64bits(v)
				appendBody("ulong tmp%d = %d; // %f", tmpidx, bits, v)
				appendBody("double stack%s = Unsafe.As<ulong, double>(ref tmp%d);", idx, tmpidx)
				tmpidx++
			}

		case operators.I32Eqz:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s == 0) ? 1 : 0;", dst, arg)
		case operators.I32Eq:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s == stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32Ne:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s != stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32LtS:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s < stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32LtU:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = ((uint)stack%s < (uint)stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32GtS:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s > stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32GtU:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = ((uint)stack%s > (uint)stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32LeS:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s <= stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32LeU:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = ((uint)stack%s <= (uint)stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32GeS:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s >= stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32GeU:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = ((uint)stack%s >= (uint)stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64Eqz:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s == 0) ? 1 : 0;", dst, arg)
		case operators.I64Eq:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s == stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64Ne:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s != stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64LtS:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s < stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64LtU:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = ((ulong)stack%s < (ulong)stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64GtS:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s > stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64GtU:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = ((ulong)stack%s > (ulong)stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64LeS:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s <= stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64LeU:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = ((ulong)stack%s <= (ulong)stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64GeS:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s >= stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64GeU:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = ((ulong)stack%s >= (ulong)stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F32Eq:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s == stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F32Ne:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s != stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F32Lt:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s < stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F32Gt:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s > stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F32Le:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s <= stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F32Ge:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s >= stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F64Eq:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s == stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F64Ne:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s != stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F64Lt:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s < stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F64Gt:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s > stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F64Le:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s <= stack%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F64Ge:
			arg1 := blockStack.PopIndex()
			arg0 := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (stack%s >= stack%s) ? 1 : 0;", dst, arg0, arg1)

		case operators.I32Clz:
			return nil, fmt.Errorf("I32Clz is not implemented")
		case operators.I32Ctz:
			return nil, fmt.Errorf("I32Ctz is not implemented")
		case operators.I32Popcnt:
			return nil, fmt.Errorf("I32Popcnt is not implemented")
		case operators.I32Add:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s += stack%s;", dst, arg)
		case operators.I32Sub:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s -= stack%s;", dst, arg)
		case operators.I32Mul:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s *= stack%s;", dst, arg)
		case operators.I32DivS:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s /= stack%s;", dst, arg)
		case operators.I32DivU:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%[1]s = (int)((uint)stack%[1]s / (uint)stack%[2]s);", dst, arg)
		case operators.I32RemS:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s %%= stack%s;", dst, arg)
		case operators.I32RemU:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%[1]s = (int)((uint)stack%[1]s %% (uint)stack%[2]s);", dst, arg)
		case operators.I32And:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s &= stack%s;", dst, arg)
		case operators.I32Or:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s |= stack%s;", dst, arg)
		case operators.I32Xor:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s ^= stack%s;", dst, arg)
		case operators.I32Shl:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s <<= stack%s;", dst, arg)
		case operators.I32ShrS:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s >>= stack%s;", dst, arg)
		case operators.I32ShrU:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%[1]s = (int)((uint)stack%[1]s >> stack%[2]s);", dst, arg)
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
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s += stack%s;", dst, arg)
		case operators.I64Sub:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s -= stack%s;", dst, arg)
		case operators.I64Mul:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s *= stack%s;", dst, arg)
		case operators.I64DivS:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s /= stack%s;", dst, arg)
		case operators.I64DivU:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%[1]s = (long)((ulong)stack%[1]s / (ulong)stack%[2]s);", dst, arg)
		case operators.I64RemS:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s %%= stack%s;", dst, arg)
		case operators.I64RemU:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%[1]s = (long)((ulong)stack%[1]s %% (ulong)stack%[2]s);", dst, arg)
		case operators.I64And:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s &= stack%s;", dst, arg)
		case operators.I64Or:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s |= stack%s;", dst, arg)
		case operators.I64Xor:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s ^= stack%s;", dst, arg)
		case operators.I64Shl:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s <<= (int)stack%s;", dst, arg)
		case operators.I64ShrS:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s >>= (int)stack%s;", dst, arg)
		case operators.I64ShrU:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%[1]s = (long)((ulong)stack%[1]s >> (int)stack%[2]s);", dst, arg)
		case operators.I64Rotl:
			return nil, fmt.Errorf("I64Rotl is not implemented")
		case operators.I64Rotr:
			return nil, fmt.Errorf("I64Rotr is not implemented")
		case operators.F32Abs:
			idx := blockStack.PeepIndex()
			appendBody("stack%[1]s = Math.Abs(stack%[1]s);", idx)
		case operators.F32Neg:
			idx := blockStack.PeepIndex()
			appendBody("stack%[1]s = -stack%[1]s;", idx)
		case operators.F32Ceil:
			idx := blockStack.PeepIndex()
			appendBody("stack%[1]s = Math.Ceil(stack%[1]s);", idx)
		case operators.F32Floor:
			idx := blockStack.PeepIndex()
			appendBody("stack%[1]s = Math.Floor(stack%[1]s);", idx)
		case operators.F32Trunc:
			idx := blockStack.PeepIndex()
			appendBody("stack%[1]s = Math.Truncate(stack%[1]s);", idx)
		case operators.F32Nearest:
			return nil, fmt.Errorf("F32Nearest is not implemented yet")
		case operators.F32Sqrt:
			idx := blockStack.PeepIndex()
			appendBody("stack%[1]s = Math.Sqrt(stack%[1]s);", idx)
		case operators.F32Add:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s += stack%s;", dst, arg)
		case operators.F32Sub:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s -= stack%s;", dst, arg)
		case operators.F32Mul:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s *= stack%s;", dst, arg)
		case operators.F32Div:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s /= stack%s;", dst, arg)
		case operators.F32Min:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%[1]s = Math.Min(stack%[1]s, stack%[2]s);", dst, arg)
		case operators.F32Max:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%[1]s = Math.Max(stack%[1]s, stack%[2]s);", dst, arg)
		case operators.F32Copysign:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%[1]s = Math.CopySign(stack%[1]s, stack%[2]s);", dst, arg)
		case operators.F64Abs:
			idx := blockStack.PeepIndex()
			appendBody("stack%[1]s = Math.Abs(stack%[1]s);", idx)
		case operators.F64Neg:
			idx := blockStack.PeepIndex()
			appendBody("stack%[1]s = -stack%[1]s;", idx)
		case operators.F64Ceil:
			idx := blockStack.PeepIndex()
			appendBody("stack%[1]s = Math.Ceil(stack%[1]s);", idx)
		case operators.F64Floor:
			idx := blockStack.PeepIndex()
			appendBody("stack%[1]s = Math.Floor(stack%[1]s);", idx)
		case operators.F64Trunc:
			idx := blockStack.PeepIndex()
			appendBody("stack%[1]s = Math.Truncate(stack%[1]s);", idx)
		case operators.F64Nearest:
			return nil, fmt.Errorf("F64Nearest is not implemented yet")
		case operators.F64Sqrt:
			idx := blockStack.PeepIndex()
			appendBody("stack%[1]s = Math.Sqrt(stack%[1]s);", idx)
		case operators.F64Add:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s += stack%s;", dst, arg)
		case operators.F64Sub:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s -= stack%s;", dst, arg)
		case operators.F64Mul:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s *= stack%s;", dst, arg)
		case operators.F64Div:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%s /= stack%s;", dst, arg)
		case operators.F64Min:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%[1]s = Math.Min(stack%[1]s, stack%[2]s);", dst, arg)
		case operators.F64Max:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%[1]s = Math.Max(stack%[1]s, stack%[2]s);", dst, arg)
		case operators.F64Copysign:
			arg := blockStack.PopIndex()
			dst := blockStack.PeepIndex()
			appendBody("stack%[1]s = Math.CopySign(stack%[1]s, stack%[2]s);", dst, arg)

		case operators.I32WrapI64:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (int)stack%s;", dst, arg)
		case operators.I32TruncSF32:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (int)Math.Truncate(stack%s);", dst, arg)
		case operators.I32TruncUF32:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (int)((uint)Math.Truncate(stack%s));", dst, arg)
		case operators.I32TruncSF64:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (int)Math.Truncate(stack%s);", dst, arg)
		case operators.I32TruncUF64:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("int stack%s = (int)((uint)Math.Truncate(stack%s));", dst, arg)
		case operators.I64ExtendSI32:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("long stack%s = (long)stack%s;", dst, arg)
		case operators.I64ExtendUI32:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("long stack%s = (long)((ulong)stack%s);", dst, arg)
		case operators.I64TruncSF32:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("long stack%s = (long)Math.Truncate(stack%s);", dst, arg)
		case operators.I64TruncUF32:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("long stack%s = (long)((ulong)Math.Truncate(stack%s));", dst, arg)
		case operators.I64TruncSF64:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("long stack%s = (long)Math.Truncate(stack%s);", dst, arg)
		case operators.I64TruncUF64:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("long stack%s = (long)((ulong)Math.Truncate(stack%s));", dst, arg)
		case operators.F32ConvertSI32:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("float stack%s = (float)stack%s;", dst, arg)
		case operators.F32ConvertUI32:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("float stack%s = (float)((uint)stack%s);", dst, arg)
		case operators.F32ConvertSI64:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("float stack%s = (float)stack%s;", dst, arg)
		case operators.F32ConvertUI64:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("float stack%s = (float)((ulong)stack%s);", dst, arg)
		case operators.F32DemoteF64:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("float stack%s = (float)stack%s;", dst, arg)
		case operators.F64ConvertSI32:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("double stack%s = (double)stack%s;", dst, arg)
		case operators.F64ConvertUI32:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("double stack%s = (double)((uint)stack%s);", dst, arg)
		case operators.F64ConvertSI64:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("double stack%s = (double)stack%s;", dst, arg)
		case operators.F64ConvertUI64:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("double stack%s = (double)((long)stack%s);", dst, arg)
		case operators.F64PromoteF32:
			arg := blockStack.PopIndex()
			dst := blockStack.PushIndex()
			appendBody("double stack%s = (double)stack%s;", dst, arg)

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
		if blockStack.HasIndex() && dis.Code[len(dis.Code)-1].Op.Code != operators.Unreachable {
			if !strings.HasPrefix(strings.TrimSpace(body[len(body)-1]), "return ") {
				idx := blockStack.PopIndex()
				appendBody(`return stack%s;`, idx)
			}
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
