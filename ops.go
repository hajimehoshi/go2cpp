// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
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

func opsToCSharp(code []byte, sig *wasm.FunctionSig, funcs []*Func, types []*Type) ([]string, error) {
	instrs, err := disasm.Disassemble(code)
	if err != nil {
		return nil, err
	}

	var body []string
	var newIdx int
	var idxStack []int

	pushStack := func() int {
		idx := newIdx
		idxStack = append(idxStack, idx)
		newIdx++
		return idx
	}
	popStack := func() int {
		idx := idxStack[len(idxStack)-1]
		idxStack = idxStack[:len(idxStack)-1]
		return idx
	}
	peepStack := func() int {
		return idxStack[len(idxStack)-1]
	}

	for _, instr := range instrs {
		switch instr.Op.Code {
		case operators.Unreachable:
			body = append(body, `Debug.Assert(false, "not reached");`)
		case operators.Nop:
			// Do nothing
		case operators.Block:
			// TODO: Implement this.
		case operators.Loop:
			// TODO: Implement this.
		case operators.If:
			popStack()
			// TODO: Implement this.
		case operators.Else:
			// TODO: Implement this.
		case operators.End:
			// TODO: Implement this.
		case operators.Br:
			// TODO: Implement this.
		case operators.BrIf:
			// TODO: Implement this.
		case operators.BrTable:
			// TODO: Implement this.
		case operators.Return:
			switch len(sig.ReturnTypes) {
			case 0:
				body = append(body, "return;")
			default:
				body = append(body, fmt.Sprintf("return stack%d;", idxStack[len(idxStack)-1]))
			}

		case operators.Call:
			f := funcs[instr.Immediates[0].(uint32)]

			args := make([]string, len(f.Type.Sig.ParamTypes))
			for i := range f.Type.Sig.ParamTypes {
				args[len(f.Type.Sig.ParamTypes)-i-1] = fmt.Sprintf("stack%d", popStack())
			}

			var ret string
			if len(f.Type.Sig.ReturnTypes) > 0 {
				ret = fmt.Sprintf("var stack%d = ", pushStack())
			}

			body = append(body, fmt.Sprintf("%s%s(%s);", ret, identifierFromString(f.Name), strings.Join(args, ", ")))
		case operators.CallIndirect:
			idx := popStack()
			typeid := instr.Immediates[0].(uint32)
			t := types[typeid]

			args := make([]string, len(t.Sig.ParamTypes))
			for i := range t.Sig.ParamTypes {
				args[len(t.Sig.ParamTypes)-i-1] = fmt.Sprintf("stack%d", popStack())
			}

			var ret string
			if len(t.Sig.ReturnTypes) > 0 {
				ret = fmt.Sprintf("var stack%d = ", pushStack())
			}

			body = append(body, fmt.Sprintf("%s((Type%d)(funcs_[table_[0][stack%d]]))(%s);", ret, typeid, idx, strings.Join(args, ", ")))

		case operators.Drop:
			popStack()
		case operators.Select:
			// TODO: Enable this after solving stack issues.
			/*cond := popStack()
			idx1 := popStack()
			idx0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("var stack%d = (stack%d == 0) ? stack%d : stack%d;", dst, instr.Immediates[0], cond, idx0, idx1))*/

		case operators.GetLocal:
			idx := pushStack()
			body = append(body, fmt.Sprintf("var stack%d = local%d;", idx, instr.Immediates[0]))
		case operators.SetLocal:
			idx := popStack()
			body = append(body, fmt.Sprintf("local%d = stack%d;", instr.Immediates[0], idx))
		case operators.TeeLocal:
			idx := peepStack()
			body = append(body, fmt.Sprintf("local%d = stack%d;", instr.Immediates[0], idx))
		case operators.GetGlobal:
			idx := pushStack()
			body = append(body, fmt.Sprintf("var stack%d = global%d;", idx, instr.Immediates[0]))
		case operators.SetGlobal:
			idx := popStack()
			body = append(body, fmt.Sprintf("global%d = stack%d;", instr.Immediates[0], idx))

		case operators.I32Load:
			// TODO: Implement this.
			pushStack()
			idx := popStack()
			body = append(body, fmt.Sprintf("int stack%d = 0 /* TODO */;", idx))
		case operators.I64Load:
			// TODO: Implement this.
			pushStack()
			idx := popStack()
			body = append(body, fmt.Sprintf("long stack%d = 0 /* TODO */;", idx))
		case operators.F32Load:
			// TODO: Implement this.
			pushStack()
			idx := popStack()
			body = append(body, fmt.Sprintf("float stack%d = 0 /* TODO */;", idx))
		case operators.F64Load:
			// TODO: Implement this.
			pushStack()
			idx := popStack()
			body = append(body, fmt.Sprintf("double stack%d = 0 /* TODO */;", idx))
		case operators.I32Load8s:
			// TODO: Implement this.
			pushStack()
			idx := popStack()
			body = append(body, fmt.Sprintf("int stack%d = 0 /* TODO */;", idx))
		case operators.I32Load8u:
			// TODO: Implement this.
			pushStack()
			idx := popStack()
			body = append(body, fmt.Sprintf("int stack%d = 0 /* TODO */;", idx))
		case operators.I32Load16s:
			// TODO: Implement this.
			pushStack()
			idx := popStack()
			body = append(body, fmt.Sprintf("int stack%d = 0 /* TODO */;", idx))
		case operators.I32Load16u:
			// TODO: Implement this.
			pushStack()
			idx := popStack()
			body = append(body, fmt.Sprintf("int stack%d = 0 /* TODO */;", idx))
		case operators.I64Load8s:
			// TODO: Implement this.
			pushStack()
			idx := popStack()
			body = append(body, fmt.Sprintf("long stack%d = 0 /* TODO */;", idx))
		case operators.I64Load8u:
			// TODO: Implement this.
			pushStack()
			idx := popStack()
			body = append(body, fmt.Sprintf("long stack%d = 0 /* TODO */;", idx))
		case operators.I64Load16s:
			// TODO: Implement this.
			pushStack()
			idx := popStack()
			body = append(body, fmt.Sprintf("long stack%d = 0 /* TODO */;", idx))
		case operators.I64Load16u:
			// TODO: Implement this.
			pushStack()
			idx := popStack()
			body = append(body, fmt.Sprintf("long stack%d = 0 /* TODO */;", idx))
		case operators.I64Load32s:
			// TODO: Implement this.
			pushStack()
			idx := popStack()
			body = append(body, fmt.Sprintf("long stack%d = 0 /* TODO */;", idx))
		case operators.I64Load32u:
			// TODO: Implement this.
			pushStack()
			idx := popStack()
			body = append(body, fmt.Sprintf("long stack%d = 0 /* TODO */;", idx))

		case operators.I32Store:
			// TODO: Implement this.
			popStack()
			popStack()
		case operators.I64Store:
			// TODO: Implement this.
			popStack()
			popStack()
		case operators.F32Store:
			// TODO: Implement this.
			popStack()
			popStack()
		case operators.F64Store:
			// TODO: Implement this.
			popStack()
			popStack()
		case operators.I32Store8:
			// TODO: Implement this.
			popStack()
			popStack()
		case operators.I32Store16:
			// TODO: Implement this.
			popStack()
			popStack()
		case operators.I64Store8:
			// TODO: Implement this.
			popStack()
			popStack()
		case operators.I64Store16:
			// TODO: Implement this.
			popStack()
			popStack()
		case operators.I64Store32:
			// TODO: Implement this.
			popStack()
			popStack()

		case operators.CurrentMemory:
			idx := pushStack()
			// TOOD: Implement this.
			_ = idx
		case operators.GrowMemory:
			// TOOD: Implement this.

		case operators.I32Const:
			idx := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = %d;", idx, instr.Immediates[0]))
		case operators.I64Const:
			idx := pushStack()
			body = append(body, fmt.Sprintf("long stack%d = %d;", idx, instr.Immediates[0]))
		case operators.F32Const:
			idx := pushStack()
			// TODO: Implement this.
			// https://docs.microsoft.com/en-us/dotnet/api/system.runtime.compilerservices.unsafe?view=netcore-3.1
			body = append(body, fmt.Sprintf("float stack%d = 0 /* TODO */;", idx))
		case operators.F64Const:
			idx := pushStack()
			// TODO: Implement this.
			body = append(body, fmt.Sprintf("double stack%d = 0 /* TODO */;", idx))

		case operators.I32Eqz:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d == 0) ? 1 : 0;", dst, arg))
		case operators.I32Eq:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d == stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.I32Ne:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d != stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.I32LtS:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d < stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.I32LtU:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = ((uint)stack%d < (uint)stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.I32GtS:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d > stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.I32GtU:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = ((uint)stack%d > (uint)stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.I32LeS:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d <= stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.I32LeU:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = ((uint)stack%d <= (uint)stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.I32GeS:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d >= stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.I32GeU:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = ((uint)stack%d >= (uint)stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.I64Eqz:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d == 0) ? 1 : 0;", dst, arg))
		case operators.I64Eq:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d == stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.I64Ne:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d != stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.I64LtS:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d < stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.I64LtU:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = ((ulong)stack%d < (ulong)stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.I64GtS:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d > stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.I64GtU:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = ((ulong)stack%d > (ulong)stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.I64LeS:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d <= stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.I64LeU:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = ((ulong)stack%d <= (ulong)stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.I64GeS:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d >= stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.I64GeU:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = ((ulong)stack%d >= (ulong)stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.F32Eq:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d == stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.F32Ne:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d != stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.F32Lt:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d < stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.F32Gt:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d > stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.F32Le:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d <= stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.F32Ge:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d >= stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.F64Eq:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d == stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.F64Ne:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d != stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.F64Lt:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d < stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.F64Gt:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d > stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.F64Le:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d <= stack%d) ? 1 : 0;", dst, arg0, arg1))
		case operators.F64Ge:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (stack%d >= stack%d) ? 1 : 0;", dst, arg0, arg1))

		case operators.I32Clz:
			return nil, fmt.Errorf("I32Clz is not implemented")
		case operators.I32Ctz:
			return nil, fmt.Errorf("I32Ctz is not implemented")
		case operators.I32Popcnt:
			return nil, fmt.Errorf("I32Popcnt is not implemented")
		case operators.I32Add:
			arg := popStack()
			dst := peepStack()
			body = append(body, fmt.Sprintf("stack%d += stack%d;", dst, arg))
		case operators.I32Sub:
			arg := popStack()
			dst := peepStack()
			body = append(body, fmt.Sprintf("stack%d -= stack%d;", dst, arg))
		case operators.I32Mul:
			arg := popStack()
			dst := peepStack()
			body = append(body, fmt.Sprintf("stack%d *= stack%d;", dst, arg))
		case operators.I32DivS:
			arg := popStack()
			dst := peepStack()
			body = append(body, fmt.Sprintf("stack%d /= stack%d;", dst, arg))
		case operators.I32DivU:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (int)((uint)stack%d / (uint)stack%d);", dst, arg0, arg1))
		case operators.I32RemS:
			arg := popStack()
			dst := peepStack()
			body = append(body, fmt.Sprintf("stack%d %%= stack%d;", dst, arg))
		case operators.I32RemU:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (int)((uint)stack%d %% (uint)stack%d);", dst, arg0, arg1))
		case operators.I32And:
			arg := popStack()
			dst := peepStack()
			body = append(body, fmt.Sprintf("stack%d &= stack%d;", dst, arg))
		case operators.I32Or:
			arg := popStack()
			dst := peepStack()
			body = append(body, fmt.Sprintf("stack%d |= stack%d;", dst, arg))
		case operators.I32Xor:
			arg := popStack()
			dst := peepStack()
			body = append(body, fmt.Sprintf("stack%d ^= stack%d;", dst, arg))
		case operators.I32Shl:
			arg := popStack()
			dst := peepStack()
			body = append(body, fmt.Sprintf("stack%d <<= stack%d;", dst, arg))
		case operators.I32ShrS:
			arg := popStack()
			dst := peepStack()
			body = append(body, fmt.Sprintf("stack%d >>= stack%d;", dst, arg))
		case operators.I32ShrU:
			arg1 := popStack()
			arg0 := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (int)((uint)stack%d >> stack%d);", dst, arg0, arg1))
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
			// TODO: Implement this
			popStack()
		case operators.I64Sub:
			// TODO: Implement this
			popStack()
		case operators.I64Mul:
			// TODO: Implement this
			popStack()
		case operators.I64DivS:
			// TODO: Implement this
			popStack()
		case operators.I64DivU:
			// TODO: Implement this
			popStack()
		case operators.I64RemS:
			// TODO: Implement this
			popStack()
		case operators.I64RemU:
			// TODO: Implement this
			popStack()
		case operators.I64And:
			// TODO: Implement this
			popStack()
		case operators.I64Or:
			// TODO: Implement this
			popStack()
		case operators.I64Xor:
			// TODO: Implement this
			popStack()
		case operators.I64Shl:
			// TODO: Implement this
			popStack()
		case operators.I64ShrS:
			// TODO: Implement this
			popStack()
		case operators.I64ShrU:
			// TODO: Implement this
			popStack()
		case operators.I64Rotl:
			return nil, fmt.Errorf("I64Rotl is not implemented")
		case operators.I64Rotr:
			return nil, fmt.Errorf("I64Rotr is not implemented")
		case operators.F32Abs:
			idx := peepStack()
			body = append(body, fmt.Sprintf("stack%[1]d = Math.Abs(stack%[1]d);", idx))
		case operators.F32Neg:
			idx := peepStack()
			body = append(body, fmt.Sprintf("stack%[1]d = -stack%[1]d;", idx))
		case operators.F32Ceil:
			idx := peepStack()
			body = append(body, fmt.Sprintf("stack%[1]d = Math.Ceil(stack%[1]d);", idx))
		case operators.F32Floor:
			idx := peepStack()
			body = append(body, fmt.Sprintf("stack%[1]d = Math.Floor(stack%[1]d);", idx))
		case operators.F32Trunc:
			idx := peepStack()
			body = append(body, fmt.Sprintf("stack%[1]d = Math.Truncate(stack%[1]d);", idx))
		case operators.F32Nearest:
			return nil, fmt.Errorf("F32Nearest is not implemented yet")
		case operators.F32Sqrt:
			idx := peepStack()
			body = append(body, fmt.Sprintf("stack%[1]d = Math.Sqrt(stack%[1]d);", idx))
		case operators.F32Add:
			arg := popStack()
			dst := peepStack()
			body = append(body, fmt.Sprintf("stack%d += stack%d;", dst, arg))
		case operators.F32Sub:
			arg := popStack()
			dst := peepStack()
			body = append(body, fmt.Sprintf("stack%d -= stack%d;", dst, arg))
		case operators.F32Mul:
			arg := popStack()
			dst := peepStack()
			body = append(body, fmt.Sprintf("stack%d *= stack%d;", dst, arg))
		case operators.F32Div:
			arg := popStack()
			dst := peepStack()
			body = append(body, fmt.Sprintf("stack%d /= stack%d;", dst, arg))
		case operators.F32Min:
			arg := popStack()
			dst := peepStack()
			body = append(body, fmt.Sprintf("stack%[1]d = Math.Min(stack%[1]d, stack%[2]d);", dst, arg))
		case operators.F32Max:
			arg := popStack()
			dst := peepStack()
			body = append(body, fmt.Sprintf("stack%[1]d = Math.Max(stack%[1]d, stack%[2]d);", dst, arg))
		case operators.F32Copysign:
			arg := popStack()
			dst := peepStack()
			body = append(body, fmt.Sprintf("stack%[1]d = Math.CopySign(stack%[1]d, stack%[2]d);", dst, arg))
		case operators.F64Abs:
			// TODO: Implement this
		case operators.F64Neg:
			// TODO: Implement this
		case operators.F64Ceil:
			// TODO: Implement this
		case operators.F64Floor:
			// TODO: Implement this
		case operators.F64Trunc:
			// TODO: Implement this
		case operators.F64Nearest:
			return nil, fmt.Errorf("F64Nearest is not implemented yet")
		case operators.F64Sqrt:
			// TODO: Implement this
		case operators.F64Add:
			// TODO: Implement this
			popStack()
		case operators.F64Sub:
			// TODO: Implement this
			popStack()
		case operators.F64Mul:
			// TODO: Implement this
			popStack()
		case operators.F64Div:
			// TODO: Implement this
			popStack()
		case operators.F64Min:
			// TODO: Implement this
			popStack()
		case operators.F64Max:
			// TODO: Implement this
			popStack()
		case operators.F64Copysign:
			// TODO: Implement this
			popStack()

		case operators.I32WrapI64:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (int)stack%d;", dst, arg))
		case operators.I32TruncSF32:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (int)Math.Truncate(stack%d);", dst, arg))
		case operators.I32TruncUF32:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (int)((uint)Math.Truncate(stack%d));", dst, arg))
		case operators.I32TruncSF64:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (int)Math.Truncate(stack%d);", dst, arg))
		case operators.I32TruncUF64:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("int stack%d = (int)((uint)Math.Truncate(stack%d));", dst, arg))
		case operators.I64ExtendSI32:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("long stack%d = (long)stack%d;", dst, arg))
		case operators.I64ExtendUI32:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("long stack%d = (long)((ulong)stack%d);", dst, arg))
		case operators.I64TruncSF32:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("long stack%d = (long)Math.Truncate(stack%d);", dst, arg))
		case operators.I64TruncUF32:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("long stack%d = (long)((ulong)Math.Truncate(stack%d));", dst, arg))
		case operators.I64TruncSF64:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("long stack%d = (long)Math.Truncate(stack%d);", dst, arg))
		case operators.I64TruncUF64:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("long stack%d = (long)((ulong)Math.Truncate(stack%d));", dst, arg))
		case operators.F32ConvertSI32:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("float stack%d = (float)stack%d;", dst, arg))
		case operators.F32ConvertUI32:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("float stack%d = (float)((uint)stack%d);", dst, arg))
		case operators.F32ConvertSI64:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("float stack%d = (float)stack%d;", dst, arg))
		case operators.F32ConvertUI64:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("float stack%d = (float)((ulong)stack%d);", dst, arg))
		case operators.F32DemoteF64:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("float stack%d = (float)stack%d;", dst, arg))
		case operators.F64ConvertSI32:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("double stack%d = (double)stack%d;", dst, arg))
		case operators.F64ConvertUI32:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("double stack%d = (double)((uint)stack%d);", dst, arg))
		case operators.F64ConvertSI64:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("double stack%d = (double)stack%d;", dst, arg))
		case operators.F64ConvertUI64:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("double stack%d = (double)((long)stack%d);", dst, arg))
		case operators.F64PromoteF32:
			arg := popStack()
			dst := pushStack()
			body = append(body, fmt.Sprintf("double stack%d = (double)stack%d;", dst, arg))

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
		// TODO: Enable this error
		/*if len(idxStack) != 0 {
			return nil, fmt.Errorf("the stack length must be 0 but %d", len(idxStack))
		}*/
	case 1:
		switch len(idxStack) {
		case 0:
			body = append(body, `throw new Exception("not reached");`)
		default:
			// TODO: The stack must be exactly 1?
			body = append(body, fmt.Sprintf("return stack%d;", idxStack[len(idxStack)-1]))
		}
	}
	return body, nil
}
