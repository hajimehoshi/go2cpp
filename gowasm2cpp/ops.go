// SPDX-License-Identifier: Apache-2.0

package gowasm2cpp

import (
	"fmt"
	"math"
	"os"
	"runtime/debug"
	"strings"

	"github.com/go-interpreter/wagon/disasm"
	"github.com/go-interpreter/wagon/wasm"
	"github.com/go-interpreter/wagon/wasm/operators"

	"github.com/hajimehoshi/go2cpp/internal/stackvar"
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
	stackvars []*stackvar.StackVars
	s         stack
	tmpindent int
}

func (b *blockStack) UnindentTemporarily() {
	b.tmpindent--
}

func (b *blockStack) IndentTemporarily() {
	b.tmpindent++
}

func (b *blockStack) varName(idx int) string {
	if b.s.Len() > 0 {
		return fmt.Sprintf("stack%d_%d", b.s.Peep(), idx)
	}
	return fmt.Sprintf("stack%d", idx)
}

func (b *blockStack) Push(btype blockType, ret string) int {
	b.types = append(b.types, btype)
	b.rets = append(b.rets, ret)
	b.stackvars = append(b.stackvars, &stackvar.StackVars{
		VarName: b.varName,
	})
	return b.s.Push()
}

func (b *blockStack) Pop() (int, blockType, string) {
	btype := b.types[len(b.types)-1]
	ret := b.rets[len(b.rets)-1]

	b.types = b.types[:len(b.types)-1]
	b.rets = b.rets[:len(b.rets)-1]
	b.stackvars = b.stackvars[:len(b.stackvars)-1]
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

func (b *blockStack) PushLhs() string {
	if b.stackvars == nil {
		b.stackvars = []*stackvar.StackVars{
			&stackvar.StackVars{
				VarName: b.varName,
			},
		}
	}
	return b.stackvars[len(b.stackvars)-1].PushLhs()
}

func (b *blockStack) PushStackVar(expr string) {
	if b.stackvars == nil {
		b.stackvars = []*stackvar.StackVars{
			&stackvar.StackVars{
				VarName: b.varName,
			},
		}
	}
	b.stackvars[len(b.stackvars)-1].Push(expr)
}

func (b *blockStack) PopStackVar() string {
	return b.stackvars[len(b.stackvars)-1].Pop()
}

func (b *blockStack) PeepStackVar() ([]string, string) {
	return b.stackvars[len(b.stackvars)-1].Peep()
}

func (b *blockStack) Empty() bool {
	if len(b.stackvars) == 0 {
		return true
	}
	return b.stackvars[len(b.stackvars)-1].Empty()
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
		if len(args) > 0 {
			str = fmt.Sprintf(str, args...)
		}
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
			ls, v := blockStack.PeepStackVar()
			for _, l := range ls {
				appendBody(l)
			}
			return fmt.Sprintf("return %s;", v)
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
				ret = blockStack.PushLhs()
				appendBody("%s %s;", t.CSharp(), ret)
			}
			blockStack.Push(blockTypeBlock, ret)
		case operators.Loop:
			var ret string
			if t := instr.Immediates[0]; t != wasm.BlockTypeEmpty {
				t := wasmTypeToReturnType(wasm.ValueType(t.(wasm.BlockType)))
				ret = blockStack.PushLhs()
				appendBody("%s %s;", t.CSharp(), ret)
			}
			l := blockStack.Push(blockTypeLoop, ret)
			appendBody("label%d:;", l)
		case operators.If:
			cond := blockStack.PopStackVar()
			var ret string
			if t := instr.Immediates[0]; t != wasm.BlockTypeEmpty {
				t := wasmTypeToReturnType(wasm.ValueType(t.(wasm.BlockType)))
				ret = blockStack.PushLhs()
				appendBody("%s %s;", t.CSharp(), ret)
			}
			appendBody("if ((%s) != 0)", cond)
			appendBody("{")
			blockStack.Push(blockTypeIf, ret)
		case operators.Else:
			if _, _, ret := blockStack.Peep(); ret != "" {
				appendBody("%s = (%s);", ret, blockStack.PopStackVar())
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
			appendBody("if ((%s) != 0)", blockStack.PopStackVar())
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
				args[len(f.Wasm.Sig.ParamTypes)-i-1] = fmt.Sprintf("(%s)", blockStack.PopStackVar())
			}

			var ret string
			if len(f.Wasm.Sig.ReturnTypes) > 0 {
				ret = fmt.Sprintf("var %s = ", blockStack.PushLhs())
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
				args[len(t.Sig.ParamTypes)-i-1] = fmt.Sprintf("(%s)", blockStack.PopStackVar())
			}

			var ret string
			if len(t.Sig.ReturnTypes) > 0 {
				ret = fmt.Sprintf("var %s = ", blockStack.PushLhs())
			}

			appendBody("%s((Type%d)(funcs_[table_[0][%s]]))(%s);", ret, typeid, idx, strings.Join(args, ", "))

		case operators.Drop:
			blockStack.PopStackVar()
		case operators.Select:
			cond := blockStack.PopStackVar()
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) != 0) ? (%s) : (%s)", cond, arg0, arg1))

		case operators.GetLocal:
			// Copy the local variable here because local variables can be modified later.
			appendBody("var %s = local%d;", blockStack.PushLhs(), instr.Immediates[0])
		case operators.SetLocal:
			lhs := fmt.Sprintf("local%d", instr.Immediates[0])
			v := blockStack.PopStackVar()
			if lhs != v {
				appendBody("%s = (%s);", lhs, v)
			}
		case operators.TeeLocal:
			ls, v := blockStack.PeepStackVar()
			for _, l := range ls {
				appendBody(l)
			}
			lhs := fmt.Sprintf("local%d", instr.Immediates[0])
			if lhs != v {
				appendBody("%s = (%s);", lhs, v)
			}
		case operators.GetGlobal:
			// Copy the global variable here because global variables can be modified later.
			appendBody("var %s = global%d;", blockStack.PushLhs(), instr.Immediates[0])
		case operators.SetGlobal:
			appendBody("global%d = (%s);", instr.Immediates[0], blockStack.PopStackVar())

		case operators.I32Load:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			appendBody("var %s = mem_.LoadInt32((%s) + %d);", blockStack.PushLhs(), addr, offset)
		case operators.I64Load:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			appendBody("var %s = mem_.LoadInt64((%s) + %d);", blockStack.PushLhs(), addr, offset)
		case operators.F32Load:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			appendBody("var %s = mem_.LoadFloat32((%s) + %d);", blockStack.PushLhs(), addr, offset)
		case operators.F64Load:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			appendBody("var %s = mem_.LoadFloat64((%s) + %d);", blockStack.PushLhs(), addr, offset)
		case operators.I32Load8s:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			appendBody("var %s = (int)mem_.LoadInt8((%s) + %d);", blockStack.PushLhs(), addr, offset)
		case operators.I32Load8u:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			appendBody("var %s = (int)mem_.LoadUint8((%s) + %d);", blockStack.PushLhs(), addr, offset)
		case operators.I32Load16s:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			appendBody("var %s = (int)mem_.LoadInt16((%s) + %d);", blockStack.PushLhs(), addr, offset)
		case operators.I32Load16u:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			appendBody("var %s = (int)mem_.LoadUint16((%s) + %d);", blockStack.PushLhs(), addr, offset)
		case operators.I64Load8s:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			appendBody("var %s = (long)mem_.LoadInt8((%s) + %d);", blockStack.PushLhs(), addr, offset)
		case operators.I64Load8u:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			appendBody("var %s = (long)mem_.LoadUint8((%s) + %d);", blockStack.PushLhs(), addr, offset)
		case operators.I64Load16s:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			appendBody("var %s = (long)mem_.LoadInt16((%s) + %d);", blockStack.PushLhs(), addr, offset)
		case operators.I64Load16u:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			appendBody("var %s = (long)mem_.LoadUint16((%s) + %d);", blockStack.PushLhs(), addr, offset)
		case operators.I64Load32s:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			appendBody("var %s = (long)mem_.LoadInt32((%s) + %d);", blockStack.PushLhs(), addr, offset)
		case operators.I64Load32u:
			offset := instr.Immediates[1].(uint32)
			addr := blockStack.PopStackVar()
			appendBody("var %s = (long)mem_.LoadUint32((%s) + %d);", blockStack.PushLhs(), addr, offset)

		case operators.I32Store:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopStackVar()
			addr := blockStack.PopStackVar()
			appendBody("mem_.StoreInt32((%s) + %d, %s);", addr, offset, idx)
		case operators.I64Store:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopStackVar()
			addr := blockStack.PopStackVar()
			appendBody("mem_.StoreInt64((%s) + %d, %s);", addr, offset, idx)
		case operators.F32Store:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopStackVar()
			addr := blockStack.PopStackVar()
			appendBody("mem_.StoreFloat32((%s) + %d, %s);", addr, offset, idx)
		case operators.F64Store:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopStackVar()
			addr := blockStack.PopStackVar()
			appendBody("mem_.StoreFloat64((%s) + %d, %s);", addr, offset, idx)
		case operators.I32Store8:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopStackVar()
			addr := blockStack.PopStackVar()
			appendBody("mem_.StoreInt8((%s) + %d, unchecked((sbyte)(%s)));", addr, offset, idx)
		case operators.I32Store16:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopStackVar()
			addr := blockStack.PopStackVar()
			appendBody("mem_.StoreInt16((%s) + %d, unchecked((short)(%s)));", addr, offset, idx)
		case operators.I64Store8:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopStackVar()
			addr := blockStack.PopStackVar()
			appendBody("mem_.StoreInt8((%s) + %d, unchecked((sbyte)(%s)));", addr, offset, idx)
		case operators.I64Store16:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopStackVar()
			addr := blockStack.PopStackVar()
			appendBody("mem_.StoreInt16((%s) + %d, unchecked((short)(%s)));", addr, offset, idx)
		case operators.I64Store32:
			offset := instr.Immediates[1].(uint32)
			idx := blockStack.PopStackVar()
			addr := blockStack.PopStackVar()
			appendBody("mem_.StoreInt32((%s) + %d, unchecked((int)(%s)));", addr, offset, idx)

		case operators.CurrentMemory:
			blockStack.PushStackVar("mem_.Size")
		case operators.GrowMemory:
			delta := blockStack.PopStackVar()
			// As Grow has side effects, call PushLhs instead of PushStackVar.
			v := blockStack.PushLhs()
			appendBody("var %s = mem_.Grow(%s);", v, delta)

		case operators.I32Const:
			blockStack.PushStackVar(fmt.Sprintf("%d", instr.Immediates[0]))
		case operators.I64Const:
			blockStack.PushStackVar(fmt.Sprintf("%dL", instr.Immediates[0]))
		case operators.F32Const:
			if v := instr.Immediates[0].(float32); v == 0 {
				blockStack.PushStackVar("0.0f");
			} else {
				va := blockStack.PushLhs()
				bits := math.Float32bits(v)
				appendBody("uint tmp%d = %d; // %f", tmpidx, bits, v)
				appendBody("float %s;", va)
				appendBody("unsafe { %s = *(float*)(&tmp%d); };", va, tmpidx)
				tmpidx++
			}
		case operators.F64Const:
			if v := instr.Immediates[0].(float64); v == 0 {
				blockStack.PushStackVar("0.0");
			} else {
				va := blockStack.PushLhs()
				bits := math.Float64bits(v)
				appendBody("ulong tmp%d = %dUL; // %f", tmpidx, bits, v)
				appendBody("double %s;", va)
				appendBody("unsafe { %s = *(double*)(&tmp%d); };", va, tmpidx)
				tmpidx++
			}

		case operators.I32Eqz:
			arg := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) == 0) ? 1 : 0", arg))
		case operators.I32Eq:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) == (%s)) ? 1 : 0", arg0, arg1))
		case operators.I32Ne:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) != (%s)) ? 1 : 0", arg0, arg1))
		case operators.I32LtS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) < (%s)) ? 1 : 0", arg0, arg1))
		case operators.I32LtU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(unchecked((uint)(%s)) < unchecked((uint)(%s))) ? 1 : 0", arg0, arg1))
		case operators.I32GtS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) > (%s)) ? 1 : 0", arg0, arg1))
		case operators.I32GtU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(unchecked((uint)(%s)) > unchecked((uint)(%s))) ? 1 : 0", arg0, arg1))
		case operators.I32LeS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) <= (%s)) ? 1 : 0", arg0, arg1))
		case operators.I32LeU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(unchecked((uint)(%s)) <= unchecked((uint)(%s))) ? 1 : 0", arg0, arg1))
		case operators.I32GeS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) >= (%s)) ? 1 : 0", arg0, arg1))
		case operators.I32GeU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(unchecked((uint)(%s)) >= unchecked((uint)(%s))) ? 1 : 0", arg0, arg1))
		case operators.I64Eqz:
			arg := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) == 0) ? 1 : 0", arg))
		case operators.I64Eq:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) == (%s)) ? 1 : 0", arg0, arg1))
		case operators.I64Ne:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) != (%s)) ? 1 : 0", arg0, arg1))
		case operators.I64LtS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) < (%s)) ? 1 : 0", arg0, arg1))
		case operators.I64LtU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(unchecked((ulong)(%s)) < unchecked((ulong)(%s))) ? 1 : 0", arg0, arg1))
		case operators.I64GtS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) > (%s)) ? 1 : 0", arg0, arg1))
		case operators.I64GtU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(unchecked((ulong)(%s)) > unchecked((ulong)(%s))) ? 1 : 0", arg0, arg1))
		case operators.I64LeS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) <= (%s)) ? 1 : 0", arg0, arg1))
		case operators.I64LeU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(unchecked((ulong)(%s)) <= unchecked((ulong)(%s))) ? 1 : 0", arg0, arg1))
		case operators.I64GeS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) >= (%s)) ? 1 : 0", arg0, arg1))
		case operators.I64GeU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(unchecked((ulong)(%s)) >= unchecked((ulong)(%s))) ? 1 : 0", arg0, arg1))
		case operators.F32Eq:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) == (%s)) ? 1 : 0", arg0, arg1))
		case operators.F32Ne:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) != (%s)) ? 1 : 0", arg0, arg1))
		case operators.F32Lt:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) < (%s)) ? 1 : 0", arg0, arg1))
		case operators.F32Gt:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) > (%s)) ? 1 : 0", arg0, arg1))
		case operators.F32Le:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) <= (%s)) ? 1 : 0", arg0, arg1))
		case operators.F32Ge:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) >= (%s)) ? 1 : 0", arg0, arg1))
		case operators.F64Eq:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) == (%s)) ? 1 : 0", arg0, arg1))
		case operators.F64Ne:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) != (%s)) ? 1 : 0", arg0, arg1))
		case operators.F64Lt:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) < (%s)) ? 1 : 0", arg0, arg1))
		case operators.F64Gt:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) > (%s)) ? 1 : 0", arg0, arg1))
		case operators.F64Le:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) <= (%s)) ? 1 : 0", arg0, arg1))
		case operators.F64Ge:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("((%s) >= (%s)) ? 1 : 0", arg0, arg1))

		case operators.I32Clz:
			arg := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("Bits.LeadingZeros(unchecked((uint)(%s)))", arg))
		case operators.I32Ctz:
			arg := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("Bits.TailingZeros(unchecked((uint)(%s)))", arg))
		case operators.I32Popcnt:
			arg := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("Bits.OnesCount(unchecked((uint)(%s)))", arg))
		case operators.I32Add:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) + (%s)", arg0, arg1))
		case operators.I32Sub:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) - (%s)", arg0, arg1))
		case operators.I32Mul:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) * (%s)", arg0, arg1))
		case operators.I32DivS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) / (%s)", arg0, arg1))
		case operators.I32DivU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(int)(unchecked((uint)(%s)) / unchecked((uint)(%s)))", arg0, arg1))
		case operators.I32RemS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) %% (%s)", arg0, arg1))
		case operators.I32RemU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(int)(unchecked((uint)(%s)) %% unchecked((uint)(%s)))", arg0, arg1))
		case operators.I32And:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) & (%s)", arg0, arg1))
		case operators.I32Or:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) | (%s)", arg0, arg1))
		case operators.I32Xor:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) ^ (%s)", arg0, arg1))
		case operators.I32Shl:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) << (%s)", arg0, arg1))
		case operators.I32ShrS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) >> (%s)", arg0, arg1))
		case operators.I32ShrU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(int)(unchecked((uint)(%s)) >> (%s))", arg0, arg1))
		case operators.I32Rotl:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(int)Bits.RotateLeft(unchecked((uint)(%s)), unchecked((int)(%s)))", arg0, arg1))
		case operators.I32Rotr:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(int)Bits.RotateLeft(unchecked((uint)(%s)), -unchecked((int)(%s)))", arg0, arg1))
		case operators.I64Clz:
			arg := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(long)Bits.LeadingZeros(unchecked((ulong)(%s)))", arg))
		case operators.I64Ctz:
			arg := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(long)Bits.TailingZeros(unchecked((ulong)(%s)))", arg))
		case operators.I64Popcnt:
			arg := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(long)Bits.OnesCount(unchecked((ulong)(%s)))", arg))
		case operators.I64Add:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) + (%s)", arg0, arg1))
		case operators.I64Sub:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) - (%s)", arg0, arg1))
		case operators.I64Mul:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) * (%s)", arg0, arg1))
		case operators.I64DivS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) / (%s)", arg0, arg1))
		case operators.I64DivU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(long)(unchecked((ulong)(%s)) / unchecked((ulong)(%s)))", arg0, arg1))
		case operators.I64RemS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) %% (%s)", arg0, arg1))
		case operators.I64RemU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(long)(unchecked((ulong)(%s)) %% unchecked((ulong)(%s)))", arg0, arg1))
		case operators.I64And:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) & (%s)", arg0, arg1))
		case operators.I64Or:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) | (%s)", arg0, arg1))
		case operators.I64Xor:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) ^ (%s)", arg0, arg1))
		case operators.I64Shl:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) << unchecked((int)(%s))", arg0, arg1))
		case operators.I64ShrS:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) >> unchecked((int)(%s))", arg0, arg1))
		case operators.I64ShrU:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(long)(unchecked((ulong)(%s)) >> unchecked((int)(%s)))", arg0, arg1))
		case operators.I64Rotl:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(long)Bits.RotateLeft(unchecked((ulong)(%s)), unchecked((int)(%s)))", arg0, arg1))
		case operators.I64Rotr:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(long)Bits.RotateLeft(unchecked((ulong)(%s)), -unchecked((int)(%s)))", arg0, arg1))
		case operators.F32Abs:
			blockStack.PushStackVar(fmt.Sprintf("Math.Abs(%s)", blockStack.PopStackVar()))
		case operators.F32Neg:
			blockStack.PushStackVar(fmt.Sprintf("-(%s)", blockStack.PopStackVar()))
		case operators.F32Ceil:
			blockStack.PushStackVar(fmt.Sprintf("Math.Ceiling(%s)", blockStack.PopStackVar()))
		case operators.F32Floor:
			blockStack.PushStackVar(fmt.Sprintf("Math.Floor(%s)", blockStack.PopStackVar()))
		case operators.F32Trunc:
			blockStack.PushStackVar(fmt.Sprintf("Math.Truncate(%s)", blockStack.PopStackVar()))
		case operators.F32Nearest:
			blockStack.PushStackVar(fmt.Sprintf("Math.Round(%s)", blockStack.PopStackVar()))
		case operators.F32Sqrt:
			blockStack.PushStackVar(fmt.Sprintf("Math.Sqrt(%s)", blockStack.PopStackVar()))
		case operators.F32Add:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) + (%s)", arg0, arg1))
		case operators.F32Sub:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) - (%s)", arg0, arg1))
		case operators.F32Mul:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) * (%s)", arg0, arg1))
		case operators.F32Div:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) / (%s)", arg0, arg1))
		case operators.F32Min:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("Math.Min((%s), (%s))", arg0, arg1))
		case operators.F32Max:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("Math.Max((%s), (%s))", arg0, arg1))
		case operators.F32Copysign:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("CopySign((%s), (%s))", arg0, arg1))
		case operators.F64Abs:
			blockStack.PushStackVar(fmt.Sprintf("Math.Abs(%s)", blockStack.PopStackVar()))
		case operators.F64Neg:
			blockStack.PushStackVar(fmt.Sprintf("-(%s)", blockStack.PopStackVar()))
		case operators.F64Ceil:
			blockStack.PushStackVar(fmt.Sprintf("Math.Ceiling(%s)", blockStack.PopStackVar()))
		case operators.F64Floor:
			blockStack.PushStackVar(fmt.Sprintf("Math.Floor(%s)", blockStack.PopStackVar()))
		case operators.F64Trunc:
			blockStack.PushStackVar(fmt.Sprintf("Math.Truncate(%s)", blockStack.PopStackVar()))
		case operators.F64Nearest:
			blockStack.PushStackVar(fmt.Sprintf("Math.Round(%s)", blockStack.PopStackVar()))
		case operators.F64Sqrt:
			blockStack.PushStackVar(fmt.Sprintf("Math.Sqrt(%s)", blockStack.PopStackVar()))
		case operators.F64Add:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) + (%s)", arg0, arg1))
		case operators.F64Sub:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) - (%s)", arg0, arg1))
		case operators.F64Mul:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) * (%s)", arg0, arg1))
		case operators.F64Div:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("(%s) / (%s)", arg0, arg1))
		case operators.F64Min:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("Math.Min((%s), (%s))", arg0, arg1))
		case operators.F64Max:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("Math.Max((%s), (%s))", arg0, arg1))
		case operators.F64Copysign:
			arg1 := blockStack.PopStackVar()
			arg0 := blockStack.PopStackVar()
			blockStack.PushStackVar(fmt.Sprintf("CopySign((%s), (%s))", arg0, arg1))

		case operators.I32WrapI64:
			blockStack.PushStackVar(fmt.Sprintf("unchecked((int)(%s))", blockStack.PopStackVar()))
		case operators.I32TruncSF32:
			blockStack.PushStackVar(fmt.Sprintf("(int)Math.Truncate(%s)", blockStack.PopStackVar()))
		case operators.I32TruncUF32:
			blockStack.PushStackVar(fmt.Sprintf("(int)((uint)Math.Truncate(%s))", blockStack.PopStackVar()))
		case operators.I32TruncSF64:
			blockStack.PushStackVar(fmt.Sprintf("(int)Math.Truncate(%s)", blockStack.PopStackVar()))
		case operators.I32TruncUF64:
			blockStack.PushStackVar(fmt.Sprintf("(int)((uint)Math.Truncate(%s))", blockStack.PopStackVar()))
		case operators.I64ExtendSI32:
			blockStack.PushStackVar(fmt.Sprintf("(long)(%s)", blockStack.PopStackVar()))
		case operators.I64ExtendUI32:
			blockStack.PushStackVar(fmt.Sprintf("(long)(unchecked((uint)(%s)))", blockStack.PopStackVar()))
		case operators.I64TruncSF32:
			blockStack.PushStackVar(fmt.Sprintf("(long)Math.Truncate(%s)", blockStack.PopStackVar()))
		case operators.I64TruncUF32:
			blockStack.PushStackVar(fmt.Sprintf("(long)((ulong)Math.Truncate(%s))", blockStack.PopStackVar()))
		case operators.I64TruncSF64:
			blockStack.PushStackVar(fmt.Sprintf("(long)Math.Truncate(%s)", blockStack.PopStackVar()))
		case operators.I64TruncUF64:
			blockStack.PushStackVar(fmt.Sprintf("(long)((ulong)Math.Truncate(%s))", blockStack.PopStackVar()))
		case operators.F32ConvertSI32:
			blockStack.PushStackVar(fmt.Sprintf("(float)(%s)", blockStack.PopStackVar()))
		case operators.F32ConvertUI32:
			blockStack.PushStackVar(fmt.Sprintf("(float)(unchecked((uint)(%s)))", blockStack.PopStackVar()))
		case operators.F32ConvertSI64:
			blockStack.PushStackVar(fmt.Sprintf("(float)(%s)", blockStack.PopStackVar()))
		case operators.F32ConvertUI64:
			blockStack.PushStackVar(fmt.Sprintf("(float)(unchecked((ulong)(%s)))", blockStack.PopStackVar()))
		case operators.F32DemoteF64:
			blockStack.PushStackVar(fmt.Sprintf("(float)(%s)", blockStack.PopStackVar()))
		case operators.F64ConvertSI32:
			blockStack.PushStackVar(fmt.Sprintf("(double)(%s)", blockStack.PopStackVar()))
		case operators.F64ConvertUI32:
			blockStack.PushStackVar(fmt.Sprintf("(double)(unchecked((uint)(%s)))", blockStack.PopStackVar()))
		case operators.F64ConvertSI64:
			blockStack.PushStackVar(fmt.Sprintf("(double)(%s)", blockStack.PopStackVar()))
		case operators.F64ConvertUI64:
			blockStack.PushStackVar(fmt.Sprintf("(double)(unchecked((ulong)(%s)))", blockStack.PopStackVar()))
		case operators.F64PromoteF32:
			blockStack.PushStackVar(fmt.Sprintf("(double)(%s)", blockStack.PopStackVar()))

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
		if !blockStack.Empty() && dis.Code[len(dis.Code)-1].Op.Code != operators.Unreachable {
			if len(body) == 0 || !strings.HasPrefix(strings.TrimSpace(body[len(body)-1]), "return ") {
				appendBody(`return %s;`, blockStack.PopStackVar())
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
