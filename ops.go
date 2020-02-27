// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/go-interpreter/wagon/wasm/operators"
)

func opsToCSharp(code []byte) ([]string, error) {
	var body []string
	for len(code) > 0 {
		c := code[0]
		code = code[1:]
		op, err := operators.New(c)
		if err != nil {
			// This should be just an argument.
			body = append(body, fmt.Sprintf("// %02x", c))
			continue
		}
		switch op.Code {
		case operators.Unreachable:
			body = append(body, `Debug.Assert(false, "not reached");`)
		case operators.Nop:
			// Do nothing
		/*case operators.GetGlobal:
			idx := code[0]
			code = code[1:]*/
		default:
			body = append(body, fmt.Sprintf("// %v", op))
		}
	}
	return body, nil
}
