// SPDX-License-Identifier: Apache-2.0

package gowasm2csharp

import (
	"fmt"
	"math"
	"os"
	"runtime/debug"
	"strings"

	"github.com/go-interpreter/wagon/disasm"
	"github.com/go-interpreter/wagon/wasm"
	"github.com/go-interpreter/wagon/wasm/operators"
)

type returnType int

const (
	returnTypeVoid returnType = iota
	returnTypeI32
	returnTypeI64
	returnTypeF32
	returnTypeF64
)

func (r returnType) CSharp() string {
	switch r {
	case returnTypeVoid:
		return "void"
	case returnTypeI32:
		return "int"
	case returnTypeI64:
		return "long"
	case returnTypeF32:
		return "float"
	case returnTypeF64:
		return "double"
	default:
		panic("not reached")
	}
}

type stack struct {
	newIdx int
	stack  []int
}

func (s *stack) Push() int {
	idx := s.newIdx
	s.stack = append(s.stack, idx)
	s.newIdx++
	return idx
}

func (s *stack) Pop() int {
	idx := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
	return idx
}

func (s *stack) Peep() int {
	return s.stack[len(s.stack)-1]
}

func (s *stack) PeepLevel(level int) (int, bool) {
	if len(s.stack) > level {
		return s.stack[len(s.stack)-1-level], true
	}
	return 0, false
}

func (s *stack) Len() int {
	return len(s.stack)
}

type blockType int

const (
	blockTypeBlock blockType = iota
	blockTypeLoop
	blockTypeIf
)

type blockStack struct {
	types     []blockType
	rets      []string
	index     []*stack
	s         stack
	tmpindent int
}

func (b *blockStack) UnindentTemporarily() {
	b.tmpindent--
}

func (b *blockStack) IndentTemporarily() {
	b.tmpindent++
}

func (b *blockStack) Push(btype blockType, ret string) int {
	b.types = append(b.types, btype)
	b.rets = append(b.rets, ret)
	b.index = append(b.index, &stack{})
	return b.s.Push()
}

func (b *blockStack) Pop() (int, blockType, string) {
	btype := b.types[len(b.types)-1]
	ret := b.rets[len(b.rets)-1]

	b.types = b.types[:len(b.types)-1]
	b.rets = b.rets[:len(b.rets)-1]
	b.index = b.index[:len(b.index)-1]
	return b.s.Pop(), btype, ret
}

func (b *blockStack) Peep() (int, blockType, string) {
	return b.s.Peep(), b.types[len(b.types)-1], b.rets[len(b.rets)-1]
}

func (b *blockStack) PeepLevel(level int) (int, blockType, bool) {
	l, ok := b.s.PeepLevel(level)
	var t blockType
	if ok {
		t = b.types[len(b.types)-1-level]
	}
	return l, t, ok
}

func (b *blockStack) Len() int {
	return b.s.Len()
}

func (b *blockStack) IndentLevel() int {
	l := 0
	for _, t := range b.types {
		if t == blockTypeIf {
			l++
		}
	}
	l += b.tmpindent
	return l
}

func (b *blockStack) PushStackVar() string {
	if b.index == nil {
		b.index = []*stack{{}}
	}
	idx := b.index[len(b.index)-1].Push()
	if b.s.Len() > 0 {
		return fmt.Sprintf("stack%d_%d", b.s.Peep(), idx)
	}
	return fmt.Sprintf("stack%d", idx)
}

func (b *blockStack) PopStackVar() string {
	idx := b.index[len(b.index)-1].Pop()
	if b.s.Len() > 0 {
		return fmt.Sprintf("stack%d_%d", b.s.Peep(), idx)
	}
	return fmt.Sprintf("stack%d", idx)
}

func (b *blockStack) PeepStackVar() string {
	idx := b.index[len(b.index)-1].Peep()
	if b.s.Len() > 0 {
		return fmt.Sprintf("stack%d_%d", b.s.Peep(), idx)
	}
	return fmt.Sprintf("stack%d", idx)
}

func (b *blockStack) HasIndex() bool {
	if len(b.index) == 0 {
		return false
	}
	return b.index[len(b.index)-1].Len() > 0
}

func (f *wasmFunc) bodyToCSharp() ([]string, error) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			debug.PrintStack()
			panic(err)
		}
	}()

	sig := f.Wasm.Sig
	funcs := f.Funcs
	types := f.Types

	dis, err := disasm.NewDisassembly(f.Wasm, f.Mod)
	if err != nil {
		return nil, err
	}

	var body []string
	blockStack := &blockStack{}
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

	gotoOrReturn := func(level int) string {
		if l, _, ok := blockStack.PeepLevel(level); ok {
			return fmt.Sprintf("goto label%d;", l)
		}
		switch len(sig.ReturnTypes) {
		case 0:
			return "return;"
		default:
			// TODO: Should this be PopStackVar?
			return fmt.Sprintf("return %s;", blockStack.PeepStackVar())
		}
	}

	for _, instr := range dis.Code {
		switch instr.Op.Code {
		case operators.Unreachable:
			appendBody(`Debug.Fail("not reached");`)
		case operators.Nop:
			// Do nothing
		case operators.Block:
			var ret string
			if t := instr.Immediates[0]; t != wasm.BlockTypeEmpty {
				t := wasmTypeToReturnType(wasm.ValueType(t.(wasm.BlockType)))
				ret = blockStack.PushStackVar()
				appendBody("%s %s;", t.CSharp(), ret)
			}
			blockStack.Push(blockTypeBlock, ret)
		case operators.Loop:
			var ret string
			if t := instr.Immediates[0]; t != wasm.BlockTypeEmpty {
				t := wasmTypeToReturnType(wasm.ValueType(t.(wasm.BlockType)))
				ret = blockStack.PushStackVar()
				appendBody("%s %s;", t.CSharp(), ret)
			}
			l := blockStack.Push(blockTypeLoop, ret)
			appendBody("label%d:;", l)
		case operators.If:
			cond := blockStack.PopStackVar()
			var ret string
			if t := instr.Immediates[0]; t != wasm.BlockTypeEmpty {
				t := wasmTypeToReturnType(wasm.ValueType(t.(wasm.BlockType)))
				ret = blockStack.PushStackVar()
				appendBody("%s %s;", t.CSharp(), ret)
			}
			appendBody("if (%s != 0)", cond)
			appendBody("{")
			blockStack.Push(blockTypeIf, ret)
		case operators.Else:
			if _, _, ret := blockStack.Peep(); ret != "" {
				idx := blockStack.PopStackVar()
				appendBody("%s = %s;", ret, idx)
			}
			blockStack.UnindentTemporarily()
			appendBody("}")
			appendBody("else")
			appendBody("{")
			blockStack.IndentTemporarily()
		case operators.End:
			if _, btype, ret := blockStack.Peep(); btype != blockTypeLoop && ret != "" {
				idx := blockStack.PopStackVar()
				appendBody("%s = %s;", ret, idx)
			}
			idx, btype, _ := blockStack.Pop()
			if btype == blockTypeIf {
				appendBody("}")
			}
			if btype != blockTypeLoop {
				appendBody("label%d:;", idx)
			}
		case operators.Br:
			if _, _, ret := blockStack.Peep(); ret != "" {
				return nil, fmt.Errorf("br with a returning value is not implemented yet")
			}
			level := instr.Immediates[0].(uint32)
			appendBody(gotoOrReturn(int(level)))
		case operators.BrIf:
			if _, _, ret := blockStack.Peep(); ret != "" {
				return nil, fmt.Errorf("br_if with a returning value is not implemented yet")
			}
			level := instr.Immediates[0].(uint32)
			appendBody("if (%s != 0)", blockStack.PopStackVar())
			appendBody("{")
			blockStack.IndentTemporarily()
			appendBody(gotoOrReturn(int(level)))
			blockStack.UnindentTemporarily()
			appendBody("}")
		case operators.BrTable:
			if _, _, ret := blockStack.Peep(); ret != "" {
				return nil, fmt.Errorf("br_table with a returning value is not implemented yet")
			}
			appendBody("switch (%s)", blockStack.PopStackVar())
			appendBody("{")
			len := int(instr.Immediates[0].(uint32))
			for i := 0; i < len; i++ {
				level := int(instr.Immediates[1+i].(uint32))
				appendBody("case %d: %s", i, gotoOrReturn(int(level)))
			}
			level := int(instr.Immediates[len+1].(uint32))
			appendBody("default: %s", gotoOrReturn(int(level)))
			appendBody("}")
		case operators.Return:
			switch len(sig.ReturnTypes) {
			case 0:
				appendBody("return;")
			default:
				appendBody("return %s;", blockStack.PopStackVar())
			}

		case operators.Call:
			f := funcs[instr.Immediates[0].(uint32)]

			args := make([]string, len(f.Wasm.Sig.ParamTypes))
			for i := range f.Wasm.Sig.ParamTypes {
				args[len(f.Wasm.Sig.ParamTypes)-i-1] = fmt.Sprintf("%s", blockStack.PopStackVar())
			}

			var ret string
			if len(f.Wasm.Sig.ReturnTypes) > 0 {
				ret = fmt.Sprintf("var %s = ", blockStack.PushStackVar())
			}

			var imp string
			if f.Import {
				imp = "import_."
			}
			appendBody("%s%s%s(%s);", ret, imp, identifierFromString(f.Wasm.Name), strings.Join(args, ", "))
		case operators.CallIndirect:
			idx := blockStack.PopStackVar()
			typeid := instr.Immediates[0].(uint32)
			t := types[typeid]

			args := make([]string, len(t.Sig.ParamTypes))
			for i := range t.Sig.ParamTypes {
				args[len(t.Sig.ParamTypes)-i-1] = fmt.Sprintf("%s", blockStack.PopStackVar())
			}

			var ret string
			if len(t.Sig.ReturnTypes) > 0 {
				ret = fmt.Sprintf("var %s = ", blockStack.PushStackVar())
			}

			appendBody("%s((Type%d)(funcs_[table_[0][%s]]))(%s);", ret, typeid, idx, strings.Join(args, ", "))

		case operators.Drop:
			blockStack.PopStackVar()
		case operators.Select:
			cond := blockStack.PopStackVar()
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PeepStackVar()
			appendBody("%[2]s = (%[1]s != 0) ? %[2]s : %[3]s;", cond, arg0, arg1)

		case operators.GetLocal:
			idx := blockStack.PushStackVar()
			appendBody("var %s = local%d;", idx, instr.Immediates[0])
		case operators.SetLocal:
			idx := blockStack.PopStackVar()
			appendBody("local%d = %s;", instr.Immediates[0], idx)
		case operators.TeeLocal:
			idx := blockStack.PeepStackVar()
			appendBody("local%d = %s;", instr.Immediates[0], idx)
		case operators.GetGlobal:
			idx := blockStack.PushStackVar()
			appendBody("var %s = global%d;", idx, instr.Immediates[0])
		case operators.SetGlobal:
			idx := blockStack.PopStackVar()
			appendBody("global%d = %s;", instr.Immediates[0], idx)

		case operators.I32Load:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			idx := blockStack.PushStackVar()
			appendBody("int %s = mem_.LoadInt32(%s + %d);", idx, addr, offset)
		case operators.I64Load:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			idx := blockStack.PushStackVar()
			appendBody("long %s = mem_.LoadInt64(%s + %d);", idx, addr, offset)
		case operators.F32Load:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			idx := blockStack.PushStackVar()
			appendBody("float %s = mem_.LoadFloat32(%s + %d);", idx, addr, offset)
		case operators.F64Load:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			idx := blockStack.PushStackVar()
			appendBody("double %s = mem_.LoadFloat64(%s + %d);", idx, addr, offset)
		case operators.I32Load8s:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			idx := blockStack.PushStackVar()
			appendBody("int %s = (int)mem_.LoadInt8(%s + %d);", idx, addr, offset)
		case operators.I32Load8u:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			idx := blockStack.PushStackVar()
			appendBody("int %s = (int)mem_.LoadUint8(%s + %d);", idx, addr, offset)
		case operators.I32Load16s:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			idx := blockStack.PushStackVar()
			appendBody("int %s = (int)mem_.LoadInt16(%s + %d);", idx, addr, offset)
		case operators.I32Load16u:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			idx := blockStack.PushStackVar()
			appendBody("int %s = (int)mem_.LoadUint16(%s + %d);", idx, addr, offset)
		case operators.I64Load8s:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			idx := blockStack.PushStackVar()
			appendBody("long %s = (long)mem_.LoadInt8(%s + %d);", idx, addr, offset)
		case operators.I64Load8u:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			idx := blockStack.PushStackVar()
			appendBody("long %s = (long)mem_.LoadUint8(%s + %d);", idx, addr, offset)
		case operators.I64Load16s:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			idx := blockStack.PushStackVar()
			appendBody("long %s = (long)mem_.LoadInt16(%s + %d);", idx, addr, offset)
		case operators.I64Load16u:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			idx := blockStack.PushStackVar()
			appendBody("long %s = (long)mem_.LoadUint16(%s + %d);", idx, addr, offset)
		case operators.I64Load32s:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			idx := blockStack.PushStackVar()
			appendBody("long %s = (long)mem_.LoadInt32(%s + %d);", idx, addr, offset)
		case operators.I64Load32u:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			idx := blockStack.PushStackVar()
			appendBody("long %s = (long)mem_.LoadUint32(%s + %d);", idx, addr, offset)

		case operators.I32Store:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopStackVar()
			addr := blockStack.PopStackVar()
			appendBody("mem_.StoreInt32(%s + %d, %s);", addr, offset, idx)
		case operators.I64Store:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopStackVar()
			addr := blockStack.PopStackVar()
			appendBody("mem_.StoreInt64(%s + %d, %s);", addr, offset, idx)
		case operators.F32Store:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopStackVar()
			addr := blockStack.PopStackVar()
			appendBody("mem_.StoreFloat32(%s + %d, %s);", addr, offset, idx)
		case operators.F64Store:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopStackVar()
			addr := blockStack.PopStackVar()
			appendBody("mem_.StoreFloat64(%s + %d, %s);", addr, offset, idx)
		case operators.I32Store8:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopStackVar()
			addr := blockStack.PopStackVar()
			appendBody("mem_.StoreInt8(%s + %d, (sbyte)%s);", addr, offset, idx)
		case operators.I32Store16:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopStackVar()
			addr := blockStack.PopStackVar()
			appendBody("mem_.StoreInt16(%s + %d, (short)%s);", addr, offset, idx)
		case operators.I64Store8:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopStackVar()
			addr := blockStack.PopStackVar()
			appendBody("mem_.StoreInt8(%s + %d, (sbyte)%s);", addr, offset, idx)
		case operators.I64Store16:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopStackVar()
			addr := blockStack.PopStackVar()
			appendBody("mem_.StoreInt16(%s + %d, (short)%s);", addr, offset, idx)
		case operators.I64Store32:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopStackVar()
			addr := blockStack.PopStackVar()
			appendBody("mem_.StoreInt32(%s + %d, (int)%s);", addr, offset, idx)

		case operators.CurrentMemory:
			appendBody("int %s = mem_.Size;", blockStack.PushStackVar())
		case operators.GrowMemory:
			delta := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = mem_.Grow(%s);", dst, delta)

		case operators.I32Const:
			idx := blockStack.PushStackVar()
			appendBody("int %s = %d;", idx, instr.Immediates[0])
		case operators.I64Const:
			idx := blockStack.PushStackVar()
			appendBody("long %s = %dL;", idx, instr.Immediates[0])
		case operators.F32Const:
			idx := blockStack.PushStackVar()
			if v := instr.Immediates[0].(float32); v == 0 {
				appendBody("float %s = 0;", idx)
			} else {
				bits := math.Float32bits(v)
				appendBody("uint tmp%d = %d; // %f", tmpidx, bits, v)
				appendBody("float %s;", idx)
				appendBody("unsafe { %s = *(float*)(&tmp%d); };", idx, tmpidx)
				tmpidx++
			}
		case operators.F64Const:
			idx := blockStack.PushStackVar()
			if v := instr.Immediates[0].(float64); v == 0 {
				appendBody("double %s = 0;", idx)
			} else {
				bits := math.Float64bits(v)
				appendBody("ulong tmp%d = %dUL; // %f", tmpidx, bits, v)
				appendBody("double %s;", idx)
				appendBody("unsafe { %s = *(double*)(&tmp%d); };", idx, tmpidx)
				tmpidx++
			}

		case operators.I32Eqz:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s == 0) ? 1 : 0;", dst, arg)
		case operators.I32Eq:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s == %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32Ne:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s != %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32LtS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s < %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32LtU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = ((uint)%s < (uint)%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32GtS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s > %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32GtU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = ((uint)%s > (uint)%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32LeS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s <= %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32LeU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = ((uint)%s <= (uint)%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32GeS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s >= %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I32GeU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = ((uint)%s >= (uint)%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64Eqz:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s == 0) ? 1 : 0;", dst, arg)
		case operators.I64Eq:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s == %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64Ne:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s != %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64LtS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s < %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64LtU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = ((ulong)%s < (ulong)%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64GtS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s > %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64GtU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = ((ulong)%s > (ulong)%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64LeS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s <= %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64LeU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = ((ulong)%s <= (ulong)%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64GeS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s >= %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.I64GeU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = ((ulong)%s >= (ulong)%s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F32Eq:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s == %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F32Ne:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s != %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F32Lt:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s < %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F32Gt:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s > %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F32Le:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s <= %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F32Ge:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s >= %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F64Eq:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s == %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F64Ne:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s != %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F64Lt:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s < %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F64Gt:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s > %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F64Le:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s <= %s) ? 1 : 0;", dst, arg0, arg1)
		case operators.F64Ge:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (%s >= %s) ? 1 : 0;", dst, arg0, arg1)

		case operators.I32Clz:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = Bits.LeadingZeros((uint)%[1]s);", idx)
		case operators.I32Ctz:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = Bits.TailingZeros((uint)%[1]s);", idx)
		case operators.I32Popcnt:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = Bits.OnesCount((uint)%[1]s);", idx)
		case operators.I32Add:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s += %s;", dst, arg)
		case operators.I32Sub:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s -= %s;", dst, arg)
		case operators.I32Mul:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s *= %s;", dst, arg)
		case operators.I32DivS:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s /= %s;", dst, arg)
		case operators.I32DivU:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%[1]s = (int)((uint)%[1]s / (uint)%[2]s);", dst, arg)
		case operators.I32RemS:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s %%= %s;", dst, arg)
		case operators.I32RemU:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%[1]s = (int)((uint)%[1]s %% (uint)%[2]s);", dst, arg)
		case operators.I32And:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s &= %s;", dst, arg)
		case operators.I32Or:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s |= %s;", dst, arg)
		case operators.I32Xor:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s ^= %s;", dst, arg)
		case operators.I32Shl:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s <<= %s;", dst, arg)
		case operators.I32ShrS:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s >>= %s;", dst, arg)
		case operators.I32ShrU:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%[1]s = (int)((uint)%[1]s >> %[2]s);", dst, arg)
		case operators.I32Rotl:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%[1]s = (int)Bits.RotateLeft((uint)%[1]s, (int)%[2]s);", dst, arg)
		case operators.I32Rotr:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%[1]s = (int)Bits.RotateLeft((uint)%[1]s, -(int)%[2]s);", dst, arg)
		case operators.I64Clz:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = (long)Bits.LeadingZeros((ulong)%[1]s);", idx)
		case operators.I64Ctz:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = (long)Bits.TailingZeros((ulong)%[1]s);", idx)
		case operators.I64Popcnt:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = (long)Bits.OnesCount((ulong)%[1]s);", idx)
		case operators.I64Add:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s += %s;", dst, arg)
		case operators.I64Sub:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s -= %s;", dst, arg)
		case operators.I64Mul:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s *= %s;", dst, arg)
		case operators.I64DivS:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s /= %s;", dst, arg)
		case operators.I64DivU:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%[1]s = (long)((ulong)%[1]s / (ulong)%[2]s);", dst, arg)
		case operators.I64RemS:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s %%= %s;", dst, arg)
		case operators.I64RemU:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%[1]s = (long)((ulong)%[1]s %% (ulong)%[2]s);", dst, arg)
		case operators.I64And:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s &= %s;", dst, arg)
		case operators.I64Or:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s |= %s;", dst, arg)
		case operators.I64Xor:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s ^= %s;", dst, arg)
		case operators.I64Shl:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s <<= (int)%s;", dst, arg)
		case operators.I64ShrS:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s >>= (int)%s;", dst, arg)
		case operators.I64ShrU:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%[1]s = (long)((ulong)%[1]s >> (int)%[2]s);", dst, arg)
		case operators.I64Rotl:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%[1]s = (long)Bits.RotateLeft((ulong)%[1]s, (int)%[2]s);", dst, arg)
		case operators.I64Rotr:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%[1]s = (long)Bits.RotateLeft((ulong)%[1]s, -(int)%[2]s);", dst, arg)
		case operators.F32Abs:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = Math.Abs(%[1]s);", idx)
		case operators.F32Neg:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = -%[1]s;", idx)
		case operators.F32Ceil:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = Math.Ceiling(%[1]s);", idx)
		case operators.F32Floor:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = Math.Floor(%[1]s);", idx)
		case operators.F32Trunc:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = Math.Truncate(%[1]s);", idx)
		case operators.F32Nearest:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = Math.Round(%[1]s);", idx)
		case operators.F32Sqrt:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = Math.Sqrt(%[1]s);", idx)
		case operators.F32Add:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s += %s;", dst, arg)
		case operators.F32Sub:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s -= %s;", dst, arg)
		case operators.F32Mul:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s *= %s;", dst, arg)
		case operators.F32Div:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s /= %s;", dst, arg)
		case operators.F32Min:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%[1]s = Math.Min(%[1]s, %[2]s);", dst, arg)
		case operators.F32Max:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%[1]s = Math.Max(%[1]s, %[2]s);", dst, arg)
		case operators.F32Copysign:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%[1]s = CopySign(%[1]s, %[2]s);", dst, arg)
		case operators.F64Abs:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = Math.Abs(%[1]s);", idx)
		case operators.F64Neg:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = -%[1]s;", idx)
		case operators.F64Ceil:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = Math.Ceiling(%[1]s);", idx)
		case operators.F64Floor:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = Math.Floor(%[1]s);", idx)
		case operators.F64Trunc:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = Math.Truncate(%[1]s);", idx)
		case operators.F64Nearest:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = Math.Round(%[1]s);", idx)
		case operators.F64Sqrt:
			idx := blockStack.PeepStackVar()
			appendBody("%[1]s = Math.Sqrt(%[1]s);", idx)
		case operators.F64Add:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s += %s;", dst, arg)
		case operators.F64Sub:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s -= %s;", dst, arg)
		case operators.F64Mul:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s *= %s;", dst, arg)
		case operators.F64Div:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%s /= %s;", dst, arg)
		case operators.F64Min:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%[1]s = Math.Min(%[1]s, %[2]s);", dst, arg)
		case operators.F64Max:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%[1]s = Math.Max(%[1]s, %[2]s);", dst, arg)
		case operators.F64Copysign:
			arg := blockStack.PopStackVar()
			dst := blockStack.PeepStackVar()
			appendBody("%[1]s = CopySign(%[1]s, %[2]s);", dst, arg)

		case operators.I32WrapI64:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (int)%s;", dst, arg)
		case operators.I32TruncSF32:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (int)Math.Truncate(%s);", dst, arg)
		case operators.I32TruncUF32:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (int)((uint)Math.Truncate(%s));", dst, arg)
		case operators.I32TruncSF64:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (int)Math.Truncate(%s);", dst, arg)
		case operators.I32TruncUF64:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("int %s = (int)((uint)Math.Truncate(%s));", dst, arg)
		case operators.I64ExtendSI32:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("long %s = (long)%s;", dst, arg)
		case operators.I64ExtendUI32:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("long %s = (long)((uint)%s);", dst, arg)
		case operators.I64TruncSF32:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("long %s = (long)Math.Truncate(%s);", dst, arg)
		case operators.I64TruncUF32:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("long %s = (long)((ulong)Math.Truncate(%s));", dst, arg)
		case operators.I64TruncSF64:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("long %s = (long)Math.Truncate(%s);", dst, arg)
		case operators.I64TruncUF64:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("long %s = (long)((ulong)Math.Truncate(%s));", dst, arg)
		case operators.F32ConvertSI32:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("float %s = (float)%s;", dst, arg)
		case operators.F32ConvertUI32:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("float %s = (float)((uint)%s);", dst, arg)
		case operators.F32ConvertSI64:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("float %s = (float)%s;", dst, arg)
		case operators.F32ConvertUI64:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("float %s = (float)((ulong)%s);", dst, arg)
		case operators.F32DemoteF64:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("float %s = (float)%s;", dst, arg)
		case operators.F64ConvertSI32:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("double %s = (double)%s;", dst, arg)
		case operators.F64ConvertUI32:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("double %s = (double)((uint)%s);", dst, arg)
		case operators.F64ConvertSI64:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("double %s = (double)%s;", dst, arg)
		case operators.F64ConvertUI64:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("double %s = (double)((long)%s);", dst, arg)
		case operators.F64PromoteF32:
			arg := blockStack.PopStackVar()
			dst := blockStack.PushStackVar()
			appendBody("double %s = (double)%s;", dst, arg)

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
				idx := blockStack.PopStackVar()
				appendBody(`return %s;`, idx)
			}
		} else {
			// Throwing an exception might prevent optimization. Use assertion here.
			appendBody(`Debug.Fail("not reached");`)
			appendBody(`return 0;`)
		}
	default:
		return nil, fmt.Errorf("unexpected num of return types: %d", len(sig.ReturnTypes))
	}

	return body, nil
}
