package main

import "fmt"

func SharedFuncA() string {
	return "shared A"
}

func SharedFuncB(x int) int {
	return x * 2
}

func SharedFuncC(a, b int) int {
	helper := func(x int) int { return x + 1 }
	return helper(a) + helper(b)
}
