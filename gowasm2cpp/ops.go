// SPDX-License-Identifier: Apache-2.0

package gowasm2cpp

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"runtime/debug"
	"sort"
	"strconv"
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

func (r returnType) Cpp() string {
	switch r {
	case returnTypeVoid:
		return "void"
	case returnTypeI32:
		return "int32_t"
	case returnTypeI64:
		return "int64_t"
	case returnTypeF32:
		return "float"
	case returnTypeF64:
		return "double"
	default:
		panic("not reached")
	}
}

func (r returnType) stackVarType() stackvar.Type {
	switch r {
	case returnTypeI32:
		return stackvar.I32
	case returnTypeI64:
		return stackvar.I64
	case returnTypeF32:
		return stackvar.F32
	case returnTypeF64:
		return stackvar.F64
	default:
		panic("not reached")
	}
}

type indexStack struct {
	newIdx int
	stack  []int
}

func (s *indexStack) Push() int {
	idx := s.newIdx
	s.stack = append(s.stack, idx)
	s.newIdx++
	return idx
}

func (s *indexStack) Pop() int {
	idx := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
	return idx
}

func (s *indexStack) Peep() int {
	return s.stack[len(s.stack)-1]
}

func (s *indexStack) PeepLevel(level int) (int, bool) {
	if len(s.stack) > level {
		return s.stack[len(s.stack)-1-level], true
	}
	return 0, false
}

func (s *indexStack) Len() int {
	return len(s.stack)
}

type blockType int

const (
	blockTypeBlock blockType = iota
	blockTypeLoop
	blockTypeIf
)

type block struct {
	typ       blockType
	ret       string
	stackvars *stackvar.StackVars
}

type blockStack struct {
	blocks     []*block
	indexstack indexStack
	tmpindent  int
}

func (b *blockStack) UnindentTemporarily() {
	b.tmpindent--
}

func (b *blockStack) IndentTemporarily() {
	b.tmpindent++
}

func (b *blockStack) blockIndex() int {
	if b.indexstack.Len() > 0 {
		return b.indexstack.Peep() + 2
	}

	// 0 is used for temporary variables.
	return 1
}

func (b *blockStack) varName(idx int) string {
	// The stack varialbe name might be replaced at aggregateStackVars later.
	// Then, the name must be easy to parse.
	return fmt.Sprintf("stack%d_%d_", b.blockIndex(), idx)
}

func (b *blockStack) PushBlock(btype blockType, ret string) int {
	b.blocks = append(b.blocks, &block{
		typ: btype,
		ret: ret,
		stackvars: &stackvar.StackVars{
			VarName: b.varName,
		},
	})
	return b.indexstack.Push()
}

func (b *blockStack) PopBlock() (id int, typ blockType, ret string) {
	bl := b.blocks[len(b.blocks)-1]
	b.blocks = b.blocks[:len(b.blocks)-1]
	return b.indexstack.Pop(), bl.typ, bl.ret
}

func (b *blockStack) PeepBlock() (id int, typ blockType, ret string) {
	bl := b.blocks[len(b.blocks)-1]
	return b.indexstack.Peep(), bl.typ, bl.ret
}

func (b *blockStack) PeepBlockLevel(level int) (id int, typ blockType, ret bool) {
	l, ok := b.indexstack.PeepLevel(level)
	var t blockType
	if ok {
		t = b.blocks[len(b.blocks)-1-level].typ
	}
	return l, t, ok
}

func (b *blockStack) Len() int {
	return b.indexstack.Len()
}

func (b *blockStack) IndentLevel() int {
	l := 0
	for _, bl := range b.blocks {
		if bl.typ == blockTypeIf {
			l++
		}
	}
	l += b.tmpindent
	return l
}

func (b *blockStack) PushLhs(t stackvar.Type) string {
	if len(b.blocks) == 0 {
		b.blocks = append(b.blocks, &block{
			stackvars: &stackvar.StackVars{
				VarName: b.varName,
			},
		})
	}
	return b.blocks[len(b.blocks)-1].stackvars.PushLhs(t)
}

func (b *blockStack) PushExpr(expr string, t stackvar.Type) {
	if len(b.blocks) == 0 {
		b.blocks = append(b.blocks, &block{
			stackvars: &stackvar.StackVars{
				VarName: b.varName,
			},
		})
	}
	b.blocks[len(b.blocks)-1].stackvars.Push(expr, t)
}

func (b *blockStack) PopExpr() (string, stackvar.Type) {
	return b.blocks[len(b.blocks)-1].stackvars.Pop()
}

func (b *blockStack) PeepExpr() ([]string, string) {
	return b.blocks[len(b.blocks)-1].stackvars.Peep()
}

func (b *blockStack) FlushExprsIfNeeded(keyword string) []string {
	if len(b.blocks) == 0 {
		return nil
	}

	sv := b.blocks[len(b.blocks)-1].stackvars
	if sv.Len() <= 1 {
		return nil
	}

	// If the keyword is not included in the exprs except for the top expr, skip flushing.
	if keyword != "" && !sv.IncludesInNonTop(keyword) {
		return nil
	}

	type exprTyp struct {
		expr string
		typ  stackvar.Type
	}
	var exprTyps []exprTyp
	for sv.Len() > 0 {
		expr, typ := sv.Pop()
		exprTyps = append(exprTyps, exprTyp{
			expr: expr,
			typ:  typ,
		})
	}

	for i := 0; i < len(exprTyps)/2; i++ {
		j := len(exprTyps) - i - 1
		exprTyps[i], exprTyps[j] = exprTyps[j], exprTyps[i]
	}

	var stmts []string
	for _, exprTyp := range exprTyps {
		stmt := fmt.Sprintf("%s %s = %s;", exprTyp.typ.Cpp(), sv.PushLhs(exprTyp.typ), exprTyp.expr)
		stmts = append(stmts, stmt)
	}

	return stmts
}

func (b *blockStack) IsStackVarEmpty() bool {
	if len(b.blocks) == 0 {
		return true
	}
	return b.blocks[len(b.blocks)-1].stackvars.Empty()
}

func (f *wasmFunc) localVariableType(idx int) returnType {
	if idx < len(f.Wasm.Sig.ParamTypes) {
		return wasmTypeToReturnType(f.Wasm.Sig.ParamTypes[idx])
	}

	idx -= len(f.Wasm.Sig.ParamTypes)
	var wt wasm.ValueType
	for _, e := range f.Wasm.Body.Locals {
		if idx >= int(e.Count) {
			idx -= int(e.Count)
			continue
		}
		wt = e.Type
		break
	}
	return wasmTypeToReturnType(wt)
}

func (f *wasmFunc) bodyToCpp() ([]string, error) {
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
		indent := strings.Repeat("  ", level)
		body = append(body, indent+str)
	}

	gotoOrReturn := func(level int) string {
		if l, _, ok := blockStack.PeepBlockLevel(level); ok {
			return fmt.Sprintf("goto label%d;", l)
		}
		switch len(sig.ReturnTypes) {
		case 0:
			return "return;"
		default:
			ls, v := blockStack.PeepExpr()
			for _, l := range ls {
				appendBody(l)
			}
			return fmt.Sprintf("return %s;", v)
		}
	}

	// Some stack variables must not be merged when they are used across multiple blocks.
	nomerge := map[string]struct{}{}

	for _, instr := range dis.Code {
		switch instr.Op.Code {
		case operators.Unreachable:
			appendBody(`assert(((void)("not reached"), false));`)
		case operators.Nop:
			// Do nothing
		case operators.Block:
			var ret string
			if t := instr.Immediates[0]; t != wasm.BlockTypeEmpty {
				return nil, fmt.Errorf("br with a returning value is not implemented yet")
			}
			blockStack.PushBlock(blockTypeBlock, ret)
		case operators.Loop:
			var ret string
			if t := instr.Immediates[0]; t != wasm.BlockTypeEmpty {
				return nil, fmt.Errorf("br with a returning value is not implemented yet")
			}
			l := blockStack.PushBlock(blockTypeLoop, ret)
			appendBody("label%d:;", l)
		case operators.If:
			cond, _ := blockStack.PopExpr()
			var ret string
			if t := instr.Immediates[0]; t != wasm.BlockTypeEmpty {
				return nil, fmt.Errorf("br with a returning value is not implemented yet")
			}
			appendBody("if (%s) {", optimizeCondition(cond))
			blockStack.PushBlock(blockTypeIf, ret)
		case operators.Else:
			if _, _, ret := blockStack.PeepBlock(); ret != "" {
				return nil, fmt.Errorf("br with a returning value is not implemented yet")
			}
			blockStack.UnindentTemporarily()
			// TODO: Treat the stack correctly especially when 'if' returns some values.
			appendBody("} else {")
			blockStack.IndentTemporarily()
		case operators.End:
			if _, _, ret := blockStack.PeepBlock(); ret != "" {
				return nil, fmt.Errorf("br with a returning value is not implemented yet")
			}
			idx, btype, _ := blockStack.PopBlock()
			if btype == blockTypeIf {
				appendBody("}")
			}
			if btype != blockTypeLoop {
				appendBody("label%d:;", idx)
			}
		case operators.Br:
			if _, _, ret := blockStack.PeepBlock(); ret != "" {
				return nil, fmt.Errorf("br with a returning value is not implemented yet")
			}
			level := instr.Immediates[0].(uint32)
			appendBody(gotoOrReturn(int(level)))
		case operators.BrIf:
			if _, _, ret := blockStack.PeepBlock(); ret != "" {
				return nil, fmt.Errorf("br_if with a returning value is not implemented yet")
			}
			level := instr.Immediates[0].(uint32)
			expr, _ := blockStack.PopExpr()
			appendBody("if (%s) {", optimizeCondition(expr))
			blockStack.IndentTemporarily()
			appendBody(gotoOrReturn(int(level)))
			blockStack.UnindentTemporarily()
			appendBody("}")
		case operators.BrTable:
			if _, _, ret := blockStack.PeepBlock(); ret != "" {
				return nil, fmt.Errorf("br_table with a returning value is not implemented yet")
			}
			expr, _ := blockStack.PopExpr()
			appendBody("switch (%s) {", expr)
			len := int(instr.Immediates[0].(uint32))
			for i := 0; i < len; i++ {
				level := int(instr.Immediates[1+i].(uint32))
				gt := gotoOrReturn(int(level))
				appendBody("case %d: %s", i, gt)
			}
			level := int(instr.Immediates[len+1].(uint32))
			appendBody("default: %s", gotoOrReturn(int(level)))
			appendBody("}")
		case operators.Return:
			switch len(sig.ReturnTypes) {
			case 0:
				appendBody("return;")
			default:
				expr, _ := blockStack.PopExpr()
				appendBody("return %s;", expr)
			}

		case operators.Call:
			f := funcs[instr.Immediates[0].(uint32)]

			args := make([]string, len(f.Wasm.Sig.ParamTypes))
			for i := range f.Wasm.Sig.ParamTypes {
				expr, _ := blockStack.PopExpr()
				args[len(f.Wasm.Sig.ParamTypes)-i-1] = fmt.Sprintf("(%s)", expr)
			}

			var ret string
			if n := len(f.Wasm.Sig.ReturnTypes); n > 0 {
				if n > 1 {
					return nil, fmt.Errorf("call: unexpected num of return types: %d", n)
				}
				t := wasmTypeToReturnType(f.Wasm.Sig.ReturnTypes[0])
				ret = fmt.Sprintf("%s %s = ", t.Cpp(), blockStack.PushLhs(t.stackVarType()))
			}

			var imp string
			if f.Import {
				imp = "import_->"
			}
			appendBody("%s%s%s(%s);", ret, imp, identifierFromString(f.Wasm.Name), strings.Join(args, ", "))
		case operators.CallIndirect:
			idx, _ := blockStack.PopExpr()
			typeid := instr.Immediates[0].(uint32)
			t := types[typeid]

			args := make([]string, len(t.Sig.ParamTypes))
			for i := range t.Sig.ParamTypes {
				expr, _ := blockStack.PopExpr()
				args[len(t.Sig.ParamTypes)-i-1] = fmt.Sprintf("(%s)", expr)
			}

			var ret string
			if n := len(t.Sig.ReturnTypes); n > 0 {
				if n > 1 {
					return nil, fmt.Errorf("call-indirect: unexpected num of return types: %d", n)
				}
				t := wasmTypeToReturnType(t.Sig.ReturnTypes[0])
				ret = fmt.Sprintf("%s %s = ", t.Cpp(), blockStack.PushLhs(t.stackVarType()))
			}

			appendBody("Type%d stack0_%d_ = funcs_[table_[0][%s]].type%d_;", typeid, tmpidx, idx, typeid)
			appendBody("%s(this->*stack0_%d_)(%s);", ret, tmpidx, strings.Join(args, ", "))
			tmpidx++

		case operators.Drop:
			blockStack.PopExpr()
		case operators.Select:
			cond, _ := blockStack.PopExpr()
			arg1, _ := blockStack.PopExpr()
			arg0, t := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) ? (%s) : (%s)", optimizeCondition(cond), arg0, arg1), t)

		case operators.GetLocal:
			t := f.localVariableType(int(instr.Immediates[0].(uint32)))
			expr := fmt.Sprintf("local%d_", instr.Immediates[0])
			blockStack.PushExpr(expr, t.stackVarType())
		case operators.SetLocal:
			lhs := fmt.Sprintf("local%d_", instr.Immediates[0])
			for _, expr := range blockStack.FlushExprsIfNeeded(lhs) {
				appendBody(expr)
			}
			v, _ := blockStack.PopExpr()
			if lhs != v {
				appendBody("%s = %s;", lhs, v)
			}
		case operators.TeeLocal:
			lhs := fmt.Sprintf("local%d_", instr.Immediates[0])
			for _, expr := range blockStack.FlushExprsIfNeeded(lhs) {
				appendBody(expr)
			}
			ls, v := blockStack.PeepExpr()
			for _, l := range ls {
				appendBody(l)
			}
			if lhs != v {
				appendBody("%s = %s;", lhs, v)
			}
		case operators.GetGlobal:
			g := f.Globals[instr.Immediates[0].(uint32)]
			t := wasmTypeToReturnType(g.Type)
			expr := fmt.Sprintf("global%d_", instr.Immediates[0])
			blockStack.PushExpr(expr, t.stackVarType())
		case operators.SetGlobal:
			lhs := fmt.Sprintf("global%d_", instr.Immediates[0])
			for _, expr := range blockStack.FlushExprsIfNeeded(lhs) {
				appendBody(expr)
			}
			expr, _ := blockStack.PopExpr()
			appendBody("%s = %s;", lhs, expr)

		case operators.I32Load:
			offset := instr.Immediates[1].(uint32)
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			expr := fmt.Sprintf("mem_->LoadInt32((%s)%s)", addr, off)
			blockStack.PushExpr(expr, stackvar.I32)
		case operators.I64Load:
			offset := instr.Immediates[1].(uint32)
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			expr := fmt.Sprintf("mem_->LoadInt64((%s)%s)", addr, off)
			blockStack.PushExpr(expr, stackvar.I64)
		case operators.F32Load:
			offset := instr.Immediates[1].(uint32)
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			expr := fmt.Sprintf("mem_->LoadFloat32((%s)%s)", addr, off)
			blockStack.PushExpr(expr, stackvar.F32)
		case operators.F64Load:
			offset := instr.Immediates[1].(uint32)
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			expr := fmt.Sprintf("mem_->LoadFloat64((%s)%s)", addr, off)
			blockStack.PushExpr(expr, stackvar.F64)
		case operators.I32Load8s:
			offset := instr.Immediates[1].(uint32)
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			expr := fmt.Sprintf("static_cast<int32_t>(mem_->LoadInt8((%s)%s))", addr, off)
			blockStack.PushExpr(expr, stackvar.I32)
		case operators.I32Load8u:
			offset := instr.Immediates[1].(uint32)
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			expr := fmt.Sprintf("static_cast<int32_t>(mem_->LoadUint8((%s)%s))", addr, off)
			blockStack.PushExpr(expr, stackvar.I32)
		case operators.I32Load16s:
			offset := instr.Immediates[1].(uint32)
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			expr := fmt.Sprintf("static_cast<int32_t>(mem_->LoadInt16((%s)%s))", addr, off)
			blockStack.PushExpr(expr, stackvar.I32)
		case operators.I32Load16u:
			offset := instr.Immediates[1].(uint32)
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			expr := fmt.Sprintf("static_cast<int32_t>(mem_->LoadUint16((%s)%s))", addr, off)
			blockStack.PushExpr(expr, stackvar.I32)
		case operators.I64Load8s:
			offset := instr.Immediates[1].(uint32)
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			expr := fmt.Sprintf("static_cast<int64_t>(mem_->LoadInt8((%s)%s))", addr, off)
			blockStack.PushExpr(expr, stackvar.I64)
		case operators.I64Load8u:
			offset := instr.Immediates[1].(uint32)
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			expr := fmt.Sprintf("static_cast<int64_t>(mem_->LoadUint8((%s)%s))", addr, off)
			blockStack.PushExpr(expr, stackvar.I64)
		case operators.I64Load16s:
			offset := instr.Immediates[1].(uint32)
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			expr := fmt.Sprintf("static_cast<int64_t>(mem_->LoadInt16((%s)%s))", addr, off)
			blockStack.PushExpr(expr, stackvar.I64)
		case operators.I64Load16u:
			offset := instr.Immediates[1].(uint32)
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			expr := fmt.Sprintf("static_cast<int64_t>(mem_->LoadUint16((%s)%s))", addr, off)
			blockStack.PushExpr(expr, stackvar.I64)
		case operators.I64Load32s:
			offset := instr.Immediates[1].(uint32)
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			expr := fmt.Sprintf("static_cast<int64_t>(mem_->LoadInt32((%s)%s))", addr, off)
			blockStack.PushExpr(expr, stackvar.I64)
		case operators.I64Load32u:
			offset := instr.Immediates[1].(uint32)
			addr, _ := blockStack.PopExpr()
			expr := fmt.Sprintf("static_cast<int64_t>(mem_->LoadUint32((%s) + %d))", addr, offset)
			blockStack.PushExpr(expr, stackvar.I64)

		case operators.I32Store:
			for _, expr := range blockStack.FlushExprsIfNeeded("mem_->") {
				appendBody(expr)
			}
			offset := instr.Immediates[1].(uint32)
			idx, _ := blockStack.PopExpr()
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			appendBody("mem_->StoreInt32((%s)%s, %s);", addr, off, idx)
		case operators.I64Store:
			for _, expr := range blockStack.FlushExprsIfNeeded("mem_->") {
				appendBody(expr)
			}
			offset := instr.Immediates[1].(uint32)
			idx, _ := blockStack.PopExpr()
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			appendBody("mem_->StoreInt64((%s)%s, %s);", addr, off, idx)
		case operators.F32Store:
			for _, expr := range blockStack.FlushExprsIfNeeded("mem_->") {
				appendBody(expr)
			}
			offset := instr.Immediates[1].(uint32)
			idx, _ := blockStack.PopExpr()
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			appendBody("mem_->StoreFloat32((%s)%s, %s);", addr, off, idx)
		case operators.F64Store:
			for _, expr := range blockStack.FlushExprsIfNeeded("mem_->") {
				appendBody(expr)
			}
			offset := instr.Immediates[1].(uint32)
			idx, _ := blockStack.PopExpr()
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			appendBody("mem_->StoreFloat64((%s)%s, %s);", addr, off, idx)
		case operators.I32Store8:
			for _, expr := range blockStack.FlushExprsIfNeeded("mem_->") {
				appendBody(expr)
			}
			offset := instr.Immediates[1].(uint32)
			idx, _ := blockStack.PopExpr()
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			appendBody("mem_->StoreInt8((%s)%s, static_cast<int8_t>(%s));", addr, off, idx)
		case operators.I32Store16:
			for _, expr := range blockStack.FlushExprsIfNeeded("mem_->") {
				appendBody(expr)
			}
			offset := instr.Immediates[1].(uint32)
			idx, _ := blockStack.PopExpr()
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			appendBody("mem_->StoreInt16((%s)%s, static_cast<int16_t>(%s));", addr, off, idx)
		case operators.I64Store8:
			for _, expr := range blockStack.FlushExprsIfNeeded("mem_->") {
				appendBody(expr)
			}
			offset := instr.Immediates[1].(uint32)
			idx, _ := blockStack.PopExpr()
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			appendBody("mem_->StoreInt8((%s)%s, static_cast<int8_t>(%s));", addr, off, idx)
		case operators.I64Store16:
			offset := instr.Immediates[1].(uint32)
			idx, _ := blockStack.PopExpr()
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			appendBody("mem_->StoreInt16((%s)%s, static_cast<int16_t>(%s));", addr, off, idx)
		case operators.I64Store32:
			for _, expr := range blockStack.FlushExprsIfNeeded("mem_->") {
				appendBody(expr)
			}
			offset := instr.Immediates[1].(uint32)
			idx, _ := blockStack.PopExpr()
			addr, _ := blockStack.PopExpr()
			var off string
			if offset != 0 {
				off = fmt.Sprintf(" + %d", offset)
			}
			appendBody("mem_->StoreInt32((%s)%s, static_cast<int32_t>(%s));", addr, off, idx)

		case operators.CurrentMemory:
			blockStack.PushExpr("mem_->GetSize()", stackvar.I32)
		case operators.GrowMemory:
			delta, _ := blockStack.PopExpr()
			// As Grow has side effects, call PushLhs instead of PushExpr.
			v := blockStack.PushLhs(stackvar.I32)
			appendBody("int32_t %s = mem_->Grow(%s);", v, delta)

		case operators.I32Const:
			blockStack.PushExpr(fmt.Sprintf("%d", instr.Immediates[0]), stackvar.I32)
		case operators.I64Const:
			if i := instr.Immediates[0].(int64); i == -9223372036854775808 {
				// C++ cannot represent this value as an integer literal.
				blockStack.PushExpr(fmt.Sprintf("%dLL - 1LL", i+1), stackvar.I64)
			} else {
				blockStack.PushExpr(fmt.Sprintf("%dLL", i), stackvar.I64)
			}
		case operators.F32Const:
			if v := instr.Immediates[0].(float32); v == 0 {
				blockStack.PushExpr("0.0f", stackvar.F32)
			} else {
				va := blockStack.PushLhs(stackvar.F32)
				bits := math.Float32bits(v)
				appendBody("uint32_t stack0_%d_ = %d; // %f", tmpidx, bits, v)
				appendBody("float %s = *reinterpret_cast<float*>(&stack0_%d_);", va, tmpidx)
				tmpidx++
			}
		case operators.F64Const:
			if v := instr.Immediates[0].(float64); v == 0 {
				blockStack.PushExpr("0.0", stackvar.I64)
			} else {
				va := blockStack.PushLhs(stackvar.F64)
				bits := math.Float64bits(v)
				appendBody("uint64_t stack0_%d_ = %dULL; // %f", tmpidx, bits, v)
				appendBody("double %s = *reinterpret_cast<double*>(&stack0_%d_);", va, tmpidx)
				tmpidx++
			}

		case operators.I32Eqz:
			arg, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) == 0", arg), stackvar.I32)
		case operators.I32Eq:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) == (%s)", arg0, arg1), stackvar.I32)
		case operators.I32Ne:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) != (%s)", arg0, arg1), stackvar.I32)
		case operators.I32LtS:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) < (%s)", arg0, arg1), stackvar.I32)
		case operators.I32LtU:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<uint32_t>(%s) < static_cast<uint32_t>(%s)", arg0, arg1), stackvar.I32)
		case operators.I32GtS:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) > (%s)", arg0, arg1), stackvar.I32)
		case operators.I32GtU:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<uint32_t>(%s) > static_cast<uint32_t>(%s)", arg0, arg1), stackvar.I32)
		case operators.I32LeS:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) <= (%s)", arg0, arg1), stackvar.I32)
		case operators.I32LeU:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<uint32_t>(%s) <= static_cast<uint32_t>(%s)", arg0, arg1), stackvar.I32)
		case operators.I32GeS:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) >= (%s)", arg0, arg1), stackvar.I32)
		case operators.I32GeU:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<uint32_t>(%s) >= static_cast<uint32_t>(%s)", arg0, arg1), stackvar.I32)
		case operators.I64Eqz:
			arg, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) == 0", arg), stackvar.I32)
		case operators.I64Eq:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) == (%s)", arg0, arg1), stackvar.I32)
		case operators.I64Ne:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) != (%s)", arg0, arg1), stackvar.I32)
		case operators.I64LtS:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) < (%s)", arg0, arg1), stackvar.I32)
		case operators.I64LtU:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<uint64_t>(%s) < static_cast<uint64_t>(%s)", arg0, arg1), stackvar.I32)
		case operators.I64GtS:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) > (%s)", arg0, arg1), stackvar.I32)
		case operators.I64GtU:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<uint64_t>(%s) > static_cast<uint64_t>(%s)", arg0, arg1), stackvar.I32)
		case operators.I64LeS:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) <= (%s)", arg0, arg1), stackvar.I32)
		case operators.I64LeU:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<uint64_t>(%s) <= static_cast<uint64_t>(%s)", arg0, arg1), stackvar.I32)
		case operators.I64GeS:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) >= (%s)", arg0, arg1), stackvar.I32)
		case operators.I64GeU:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<uint64_t>(%s) >= static_cast<uint64_t>(%s)", arg0, arg1), stackvar.I32)
		case operators.F32Eq:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) == (%s)", arg0, arg1), stackvar.I32)
		case operators.F32Ne:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) != (%s)", arg0, arg1), stackvar.I32)
		case operators.F32Lt:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) < (%s)", arg0, arg1), stackvar.I32)
		case operators.F32Gt:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) > (%s)", arg0, arg1), stackvar.I32)
		case operators.F32Le:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) <= (%s)", arg0, arg1), stackvar.I32)
		case operators.F32Ge:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) >= (%s)", arg0, arg1), stackvar.I32)
		case operators.F64Eq:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) == (%s)", arg0, arg1), stackvar.I32)
		case operators.F64Ne:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) != (%s)", arg0, arg1), stackvar.I32)
		case operators.F64Lt:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) < (%s)", arg0, arg1), stackvar.I32)
		case operators.F64Gt:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) > (%s)", arg0, arg1), stackvar.I32)
		case operators.F64Le:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) <= (%s)", arg0, arg1), stackvar.I32)
		case operators.F64Ge:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) >= (%s)", arg0, arg1), stackvar.I32)

		case operators.I32Clz:
			arg, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("Bits::LeadingZeros(static_cast<uint32_t>(%s))", arg), stackvar.I32)
		case operators.I32Ctz:
			arg, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("Bits::TailingZeros(static_cast<uint32_t>(%s))", arg), stackvar.I32)
		case operators.I32Popcnt:
			arg, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("Bits::OnesCount(static_cast<uint32_t>(%s))", arg), stackvar.I32)
		case operators.I32Add:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) + (%s)", arg0, arg1), stackvar.I32)
		case operators.I32Sub:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) - (%s)", arg0, arg1), stackvar.I32)
		case operators.I32Mul:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) * (%s)", arg0, arg1), stackvar.I32)
		case operators.I32DivS:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) / (%s)", arg0, arg1), stackvar.I32)
		case operators.I32DivU:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int32_t>(static_cast<uint32_t>(%s) / static_cast<uint32_t>(%s))", arg0, arg1), stackvar.I32)
		case operators.I32RemS:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) %% (%s)", arg0, arg1), stackvar.I32)
		case operators.I32RemU:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int32_t>(static_cast<uint32_t>(%s) %% static_cast<uint32_t>(%s))", arg0, arg1), stackvar.I32)
		case operators.I32And:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) & (%s)", arg0, arg1), stackvar.I32)
		case operators.I32Or:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) | (%s)", arg0, arg1), stackvar.I32)
		case operators.I32Xor:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) ^ (%s)", arg0, arg1), stackvar.I32)
		case operators.I32Shl:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) << (%s)", arg0, arg1), stackvar.I32)
		case operators.I32ShrS:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) >> (%s)", arg0, arg1), stackvar.I32)
		case operators.I32ShrU:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int32_t>(static_cast<uint32_t>(%s) >> (%s))", arg0, arg1), stackvar.I32)
		case operators.I32Rotl:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int32_t>(Bits::RotateLeft(static_cast<uint32_t>(%s), static_cast<int32_t>(%s)))", arg0, arg1), stackvar.I32)
		case operators.I32Rotr:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int32_t>(Bits::RotateLeft(static_cast<uint32_t>(%s), -static_cast<int32_t>(%s)))", arg0, arg1), stackvar.I32)
		case operators.I64Clz:
			arg, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int64_t>(Bits::LeadingZeros(static_cast<uint64_t>(%s)))", arg), stackvar.I64)
		case operators.I64Ctz:
			arg, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int64_t>(Bits::TailingZeros(static_cast<uint64_t>(%s)))", arg), stackvar.I64)
		case operators.I64Popcnt:
			arg, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int64_t>(Bits::OnesCount(static_cast<uint64_t>(%s)))", arg), stackvar.I64)
		case operators.I64Add:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) + (%s)", arg0, arg1), stackvar.I64)
		case operators.I64Sub:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) - (%s)", arg0, arg1), stackvar.I64)
		case operators.I64Mul:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) * (%s)", arg0, arg1), stackvar.I64)
		case operators.I64DivS:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) / (%s)", arg0, arg1), stackvar.I64)
		case operators.I64DivU:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int64_t>(static_cast<uint64_t>(%s) / static_cast<uint64_t>(%s))", arg0, arg1), stackvar.I64)
		case operators.I64RemS:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) %% (%s)", arg0, arg1), stackvar.I64)
		case operators.I64RemU:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int64_t>(static_cast<uint64_t>(%s) %% static_cast<uint64_t>(%s))", arg0, arg1), stackvar.I64)
		case operators.I64And:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) & (%s)", arg0, arg1), stackvar.I64)
		case operators.I64Or:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) | (%s)", arg0, arg1), stackvar.I64)
		case operators.I64Xor:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) ^ (%s)", arg0, arg1), stackvar.I64)
		case operators.I64Shl:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) << static_cast<int32_t>(%s)", arg0, arg1), stackvar.I64)
		case operators.I64ShrS:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) >> static_cast<int32_t>(%s)", arg0, arg1), stackvar.I64)
		case operators.I64ShrU:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int64_t>(static_cast<uint64_t>(%s) >> static_cast<int32_t>(%s))", arg0, arg1), stackvar.I64)
		case operators.I64Rotl:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int64_t>(Bits::RotateLeft(static_cast<uint64_t>(%s), static_cast<int32_t>(%s)))", arg0, arg1), stackvar.I64)
		case operators.I64Rotr:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int64_t>(Bits::RotateLeft(static_cast<uint64_t>(%s), -(static_cast<int32_t>(%s))))", arg0, arg1), stackvar.I64)
		case operators.F32Abs:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("std::abs(%s)", expr), stackvar.F32)
		case operators.F32Neg:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("-(%s)", expr), stackvar.F32)
		case operators.F32Ceil:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("std::ceil(%s)", expr), stackvar.F32)
		case operators.F32Floor:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("std::floor(%s)", expr), stackvar.F32)
		case operators.F32Trunc:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("std::trunc(%s)", expr), stackvar.F32)
		case operators.F32Nearest:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("Math::Round(%s)", expr), stackvar.F32)
		case operators.F32Sqrt:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("std::sqrt(%s)", expr), stackvar.F32)
		case operators.F32Add:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) + (%s)", arg0, arg1), stackvar.F32)
		case operators.F32Sub:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) - (%s)", arg0, arg1), stackvar.F32)
		case operators.F32Mul:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) * (%s)", arg0, arg1), stackvar.F32)
		case operators.F32Div:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) / (%s)", arg0, arg1), stackvar.F32)
		case operators.F32Min:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("std::min((%s), (%s))", arg0, arg1), stackvar.F32)
		case operators.F32Max:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("std::max((%s), (%s))", arg0, arg1), stackvar.F32)
		case operators.F32Copysign:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("std::copysign((%s), (%s))", arg0, arg1), stackvar.F32)
		case operators.F64Abs:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("std::abs(%s)", expr), stackvar.F64)
		case operators.F64Neg:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("-(%s)", expr), stackvar.F64)
		case operators.F64Ceil:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("std::ceil(%s)", expr), stackvar.F64)
		case operators.F64Floor:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("std::floor(%s)", expr), stackvar.F64)
		case operators.F64Trunc:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("std::trunc(%s)", expr), stackvar.F64)
		case operators.F64Nearest:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("Math::Round(%s)", expr), stackvar.F64)
		case operators.F64Sqrt:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("std::sqrt(%s)", expr), stackvar.F64)
		case operators.F64Add:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) + (%s)", arg0, arg1), stackvar.F64)
		case operators.F64Sub:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) - (%s)", arg0, arg1), stackvar.F64)
		case operators.F64Mul:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) * (%s)", arg0, arg1), stackvar.F64)
		case operators.F64Div:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("(%s) / (%s)", arg0, arg1), stackvar.F64)
		case operators.F64Min:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("std::min((%s), (%s))", arg0, arg1), stackvar.F64)
		case operators.F64Max:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("std::max((%s), (%s))", arg0, arg1), stackvar.F64)
		case operators.F64Copysign:
			arg1, _ := blockStack.PopExpr()
			arg0, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("std::copysign((%s), (%s))", arg0, arg1), stackvar.F64)

		case operators.I32WrapI64:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int32_t>(%s)", expr), stackvar.I32)
		case operators.I32TruncSF32:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int32_t>(std::trunc(%s))", expr), stackvar.I32)
		case operators.I32TruncUF32:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int32_t>(static_cast<uint32_t>(std::trunc(%s)))", expr), stackvar.I32)
		case operators.I32TruncSF64:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int32_t>(std::trunc(%s))", expr), stackvar.I32)
		case operators.I32TruncUF64:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int32_t>(static_cast<uint32_t>(std::trunc(%s)))", expr), stackvar.I32)
		case operators.I64ExtendSI32:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int64_t>(%s)", expr), stackvar.I64)
		case operators.I64ExtendUI32:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int64_t>(static_cast<uint32_t>(%s))", expr), stackvar.I64)
		case operators.I64TruncSF32:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int64_t>(std::trunc(%s))", expr), stackvar.I64)
		case operators.I64TruncUF32:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int64_t>(static_cast<uint64_t>(std::trunc(%s)))", expr), stackvar.I64)
		case operators.I64TruncSF64:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int64_t>(std::trunc(%s))", expr), stackvar.I64)
		case operators.I64TruncUF64:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<int64_t>(static_cast<uint64_t>(std::trunc(%s)))", expr), stackvar.I64)
		case operators.F32ConvertSI32:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<float>(%s)", expr), stackvar.F32)
		case operators.F32ConvertUI32:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<float>(static_cast<uint32_t>(%s))", expr), stackvar.F32)
		case operators.F32ConvertSI64:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<float>(%s)", expr), stackvar.F32)
		case operators.F32ConvertUI64:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<float>(static_cast<uint64_t>((%s)))", expr), stackvar.F32)
		case operators.F32DemoteF64:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<float>(%s)", expr), stackvar.F32)
		case operators.F64ConvertSI32:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<double>(%s)", expr), stackvar.F64)
		case operators.F64ConvertUI32:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<double>(static_cast<uint32_t>(%s))", expr), stackvar.F64)
		case operators.F64ConvertSI64:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<double>(%s)", expr), stackvar.F64)
		case operators.F64ConvertUI64:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<double>(static_cast<uint64_t>(%s))", expr), stackvar.F64)
		case operators.F64PromoteF32:
			expr, _ := blockStack.PopExpr()
			blockStack.PushExpr(fmt.Sprintf("static_cast<double>(%s)", expr), stackvar.F64)

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
		if !blockStack.IsStackVarEmpty() && dis.Code[len(dis.Code)-1].Op.Code != operators.Unreachable {
			if len(body) == 0 || !strings.HasPrefix(strings.TrimSpace(body[len(body)-1]), "return ") {
				expr, _ := blockStack.PopExpr()
				appendBody(`return %s;`, expr)
			}
		} else {
			// Throwing an exception might prevent optimization. Use assertion here.
			appendBody(`assert(((void)("not reached"), false));`)
			appendBody(`return 0;`)
		}
	default:
		return nil, fmt.Errorf("unexpected num of return types: %d", len(sig.ReturnTypes))
	}

	body = aggregateStackVars(body, nomerge)
	body = optimizeGoto(body)
	body = removeUnusedLabels(body)

	return body, nil
}

var (
	stackVarRe     = regexp.MustCompile(`stack[0-9]+_[0-9]+_`)
	stackVarDeclRe = regexp.MustCompile(`^\s*((int32_t|int64_t|uint32_t|uint64_t|float|double|Type[0-9]+) (stack([0-9]+)_[0-9]+_))`)
)

func aggregateStackVars(body []string, nomerge map[string]struct{}) []string {
	// To avoid "jump bypasses variable initialization" errors, all the stack variables must be declared first.

	newVarName := func(t string, idx int) string {
		var tname string
		switch t {
		case "int32_t":
			tname = "i32"
		case "int64_t":
			tname = "i64"
		case "uint32_t":
			tname = "u32"
		case "uint64_t":
			tname = "u64"
		case "float":
			tname = "f32"
		case "double":
			tname = "f64"
		default:
			tname = "t" + t[4:]
		}
		return fmt.Sprintf("%s_%d_", tname, idx)
	}

	types := map[int]map[string]int{}
	varnum := map[string]int{}
	varmap := map[string]string{}
	var nomergelines []string
	for i, l := range body {
		m := stackVarDeclRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}

		if _, ok := nomerge[m[3]]; ok {
			nomergelines = append(nomergelines, body[i])
			varmap[m[3]] = m[3]
			body[i] = ""
			continue
		}

		t := m[2]
		grp, _ := strconv.Atoi(m[4])

		if _, ok := types[grp]; !ok {
			types[grp] = map[string]int{}
		}
		newidx := types[grp][t]
		types[grp][t]++
		if varnum[t] < newidx+1 {
			varnum[t] = newidx + 1
		}

		varmap[m[3]] = newVarName(t, newidx)

		body[i] = strings.Replace(body[i], m[1], m[3], 1)
		// If the line consists of only a variable name and a semicolon after replacing, remove this.
		if strings.TrimSpace(body[i]) == m[3]+";" {
			body[i] = ""
		}
	}

	for i, l := range body {
		body[i] = stackVarRe.ReplaceAllStringFunc(l, func(from string) string {
			return varmap[from]
		})
	}

	var decls []string
	var ts []string
	for t := range varnum {
		ts = append(ts, t)
	}
	sort.Strings(ts)
	for _, t := range ts {
		c := varnum[t]
		for i := 0; i < c; i++ {
			decls = append(decls, fmt.Sprintf("  %s %s;", t, newVarName(t, i)))
		}
	}

	r := append(decls, nomergelines...)
	r = append(r, "")
	r = append(r, body...)
	return r
}

var (
	labelRe        = regexp.MustCompile(`^\s*(label\d+):;$`)
	gotoRe         = regexp.MustCompile(`^\s*((case \d+|default):\s*)?goto (label\d+);$`)
	caseGotoRe     = regexp.MustCompile(`^\s*(case (\d+)|default): goto (label\d+);$`)
	returnRe       = regexp.MustCompile(`^\s*(return.*);$`)
	brtableBeginRe = regexp.MustCompile(`^\s*switch \((local\d+_)\) {$`)
)

func optimizeGoto(body []string) []string {
	labelWithReturn := map[string]string{}

	for i, l := range body {
		m1 := labelRe.FindStringSubmatch(l)
		if m1 == nil {
			continue
		}

		label := m1[1]
		if len(body) <= i+1 {
			continue
		}

		m2 := returnRe.FindStringSubmatch(body[i+1])
		if m2 == nil {
			continue
		}
		labelWithReturn[label] = m2[1]
	}

	for i, l := range body {
		m := gotoRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		ret, ok := labelWithReturn[m[3]]
		if !ok {
			continue
		}
		var idt string
		for _, r := range l {
			if r != ' ' {
				break
			}
			idt += string(r)
		}
		body[i] = idt + ret + ";"
	}

	var brtableLocal string
	var brtableStartLabel string
	var brtableDefaultDst string
	var brtableValueToDst map[int]string

	for i := 0; i < len(body); i++ {
		l := body[i]
		m1 := brtableBeginRe.FindStringSubmatch(l)
		if m1 == nil {
			continue
		}

		m2 := labelRe.FindStringSubmatch(body[i-1])
		if m2 == nil {
			continue
		}

		brtableLocal = m1[1]
		brtableStartLabel = m2[1]
		brtableValueToDst = map[int]string{}

		i++
		for {
			m := caseGotoRe.FindStringSubmatch(body[i])
			if m == nil {
				break
			}
			if m[1] == "default" {
				brtableDefaultDst = m[3]
				break
			}
			v, err := strconv.Atoi(m[2])
			if err != nil {
				panic(err)
			}
			brtableValueToDst[v] = m[3]
			i++
		}
		break
	}

	if brtableStartLabel != "" {
		assignRe := regexp.MustCompile(`\s*` + regexp.QuoteMeta(brtableLocal) + ` = (\d+);`)
		gotoRe := regexp.MustCompile(`\s*goto ` + regexp.QuoteMeta(brtableStartLabel) + `;`)
		for i, l := range body {
			m := assignRe.FindStringSubmatch(l)
			if m == nil {
				continue
			}
			v, err := strconv.Atoi(m[1])
			if err != nil {
				panic(err)
			}
			if !gotoRe.MatchString(body[i+1]) {
				continue
			}

			var idt string
			for _, r := range body[i+1] {
				if r != ' ' {
					break
				}
				idt += string(r)
			}

			if dst, ok := brtableValueToDst[v]; ok {
				body[i+1] = idt + "goto " + dst + ";"
			} else {
				body[i+1] = idt + "goto " + brtableDefaultDst + ";"
			}
		}
	}

	return body
}

func removeUnusedLabels(body []string) []string {
	labels := map[string]int{}
	gotos := map[string]struct{}{}
	for i, l := range body {
		if m := labelRe.FindStringSubmatch(l); m != nil {
			labels[m[1]] = i
		}
		if m := gotoRe.FindStringSubmatch(l); m != nil {
			gotos[m[3]] = struct{}{}
		}
	}

	unused := map[int]struct{}{}
	for l, i := range labels {
		if _, ok := gotos[l]; ok {
			continue
		}
		unused[i] = struct{}{}
	}

	r := make([]string, 0, len(body)-len(unused))
	for i, l := range body {
		if _, ok := unused[i]; ok {
			continue
		}
		r = append(r, l)
	}

	return r
}

func hasOuterParen(str string) bool {
	if str[0] != '(' || str[len(str)-1] != ')' {
		return false
	}

	count := 1
	for _, r := range str[1 : len(str)-1] {
		switch r {
		case '(':
			count++
		case ')':
			count--
		}
		if count == 0 {
			return false
		}
	}

	return true
}

func optimizeCondition(cond string) string {
	for {
		const (
			equalToZero    = " == 0"
			notEqualToZero = " != 0"
			i32cast        = "static_cast<int32_t>"
			i64cast        = "static_cast<int64_t>"
			u32cast        = "static_cast<uint32_t>"
			u64cast        = "static_cast<uint64_t>"
		)

		if hasOuterParen(cond) {
			cond = cond[1 : len(cond)-1]
			continue
		}

		if strings.HasSuffix(cond, equalToZero) {
			cond = cond[:len(cond)-len(equalToZero)]
			cond = optimizeCondition(cond)
			if cond[0] == '!' {
				cond = cond[1:]
			} else {
				cond = "!(" + cond + ")"
			}
			continue
		}

		if strings.HasSuffix(cond, notEqualToZero) {
			cond = cond[:len(cond)-len(notEqualToZero)]
			continue
		}

		if strings.HasPrefix(cond, i32cast) && hasOuterParen(cond[len(i32cast):]) {
			cond = cond[len(i32cast)+1 : len(cond)-1]
			continue
		}
		if strings.HasPrefix(cond, i64cast) && hasOuterParen(cond[len(i64cast):]) {
			cond = cond[len(i64cast)+1 : len(cond)-1]
			continue
		}
		if strings.HasPrefix(cond, u32cast) && hasOuterParen(cond[len(u32cast):]) {
			cond = cond[len(u32cast)+1 : len(cond)-1]
			continue
		}
		if strings.HasPrefix(cond, u64cast) && hasOuterParen(cond[len(u64cast):]) {
			cond = cond[len(u64cast)+1 : len(cond)-1]
			continue
		}

		break
	}
	return cond
}
