// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/go-interpreter/wagon/disasm"
)

func opsToCSharp(code []byte) ([]string, error) {
	insts, err := disasm.Disassemble(code)
	if err != nil {
		return nil, err
	}

	var body []string
	for _, inst := range insts {
		body = append(body, fmt.Sprintf("// %v", inst.Op))
	}
	return body, nil
}
