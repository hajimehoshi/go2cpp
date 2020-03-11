// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"time"
)

func main() {
	ch1 := make(chan string)
	ch2 := make(chan string)
	ch3 := make(chan struct{})

	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			ch1 <- "A"
		}
	}()
	go func() {
		for {
			time.Sleep(200 * time.Millisecond)
			ch2 <- "B"
		}
	}()
	go func() {
		for {
			time.Sleep(time.Second)
			close(ch3)
		}
	}()

loop:
	for {
		select {
		case str := <-ch1:
			fmt.Println(str)
		case str := <-ch2:
			fmt.Println(str)
		case <-ch3:
			fmt.Println("Done")
			break loop
		}
	}
}
