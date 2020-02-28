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

	// TODO: Replace 'dynamic' with proper types

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
				ret = fmt.Sprintf("dynamic stack%d = ", pushStack())
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
				ret = fmt.Sprintf("dynamic stack%d = ", pushStack())
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
			body = append(body, fmt.Sprintf("dynamic stack%d = local%d;", idx, instr.Immediates[0]))
		case operators.SetLocal:
			idx := popStack()
			body = append(body, fmt.Sprintf("local%d = stack%d;", instr.Immediates[0], idx))
		case operators.TeeLocal:
			idx := peepStack()
			body = append(body, fmt.Sprintf("local%d = stack%d;", instr.Immediates[0], idx))
		case operators.GetGlobal:
			idx := pushStack()
			body = append(body, fmt.Sprintf("dynamic stack%d = global%d;", idx, instr.Immediates[0]))
		case operators.SetGlobal:
			idx := popStack()
			body = append(body, fmt.Sprintf("global%d = stack%d;", instr.Immediates[0], idx))

		case operators.I32Load:
			// TODO: Implement this.
		case operators.I64Load:
			// TODO: Implement this.
		case operators.F32Load:
			// TODO: Implement this.
		case operators.F64Load:
			// TODO: Implement this.
		case operators.I32Load8s:
			// TODO: Implement this.
		case operators.I32Load8u:
			// TODO: Implement this.
		case operators.I32Load16s:
			// TODO: Implement this.
		case operators.I32Load16u:
			// TODO: Implement this.
		case operators.I64Load8s:
			// TODO: Implement this.
		case operators.I64Load8u:
			// TODO: Implement this.
		case operators.I64Load16s:
			// TODO: Implement this.
		case operators.I64Load16u:
			// TODO: Implement this.
		case operators.I64Load32s:
			// TODO: Implement this.
		case operators.I64Load32u:
			// TODO: Implement this.

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
			body = append(body, fmt.Sprintf("dynamic stack%d = %d;", idx, instr.Immediates[0]))
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
			// TODO: Implement this
		case operators.I32Ctz:
			// TODO: Implement this
		case operators.I32Popcnt:
			// TODO: Implement this
		case operators.I32Add:
			// TODO: Implement this
			popStack()
		case operators.I32Sub:
			// TODO: Implement this
			popStack()
		case operators.I32Mul:
			// TODO: Implement this
			popStack()
		case operators.I32DivS:
			// TODO: Implement this
			popStack()
		case operators.I32DivU:
			// TODO: Implement this
			popStack()
		case operators.I32RemS:
			// TODO: Implement this
			popStack()
		case operators.I32RemU:
			// TODO: Implement this
			popStack()
		case operators.I32And:
			// TODO: Implement this
			popStack()
		case operators.I32Or:
			// TODO: Implement this
			popStack()
		case operators.I32Xor:
			// TODO: Implement this
			popStack()
		case operators.I32Shl:
			// TODO: Implement this
			popStack()
		case operators.I32ShrS:
			// TODO: Implement this
			popStack()
		case operators.I32ShrU:
			// TODO: Implement this
			popStack()
		case operators.I32Rotl:
			// TODO: Implement this
			popStack()
		case operators.I32Rotr:
			// TODO: Implement this
			popStack()
		case operators.I64Clz:
			// TODO: Implement this
		case operators.I64Ctz:
			// TODO: Implement this
		case operators.I64Popcnt:
			// TODO: Implement this
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
			// TODO: Implement this
			popStack()
		case operators.I64Rotr:
			// TODO: Implement this
			popStack()
		case operators.F32Abs:
			// TODO: Implement this
		case operators.F32Neg:
			// TODO: Implement this
		case operators.F32Ceil:
			// TODO: Implement this
		case operators.F32Floor:
			// TODO: Implement this
		case operators.F32Trunc:
			// TODO: Implement this
		case operators.F32Nearest:
			// TODO: Implement this
		case operators.F32Sqrt:
			// TODO: Implement this
		case operators.F32Add:
			// TODO: Implement this
			popStack()
		case operators.F32Sub:
			// TODO: Implement this
			popStack()
		case operators.F32Mul:
			// TODO: Implement this
			popStack()
		case operators.F32Div:
			// TODO: Implement this
			popStack()
		case operators.F32Min:
			// TODO: Implement this
			popStack()
		case operators.F32Max:
			// TODO: Implement this
			popStack()
		case operators.F32Copysign:
			// TODO: Implement this
			popStack()
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
			// TODO: Implement this
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
			// TODO: Implement this
		case operators.I32TruncSF32:
			// TODO: Implement this
		case operators.I32TruncUF32:
			// TODO: Implement this
		case operators.I32TruncSF64:
			// TODO: Implement this
		case operators.I32TruncUF64:
			// TODO: Implement this
		case operators.I64ExtendSI32:
			// TODO: Implement this
		case operators.I64ExtendUI32:
			// TODO: Implement this
		case operators.I64TruncSF32:
			// TODO: Implement this
		case operators.I64TruncUF32:
			// TODO: Implement this
		case operators.I64TruncSF64:
			// TODO: Implement this
		case operators.I64TruncUF64:
			// TODO: Implement this
		case operators.F32ConvertSI32:
			// TODO: Implement this
		case operators.F32ConvertUI32:
			// TODO: Implement this
		case operators.F32ConvertSI64:
			// TODO: Implement this
		case operators.F32ConvertUI64:
			// TODO: Implement this
		case operators.F32DemoteF64:
			// TODO: Implement this
		case operators.F64ConvertSI32:
			// TODO: Implement this
		case operators.F64ConvertUI32:
			// TODO: Implement this
		case operators.F64ConvertSI64:
			// TODO: Implement this
		case operators.F64ConvertUI64:
			// TODO: Implement this
		case operators.F64PromoteF32:
			// TODO: Implement this

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
