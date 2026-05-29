package main

import "fmt"

// Hello 一个简单的函数
func Hello(name string) string {
	msg := Greet(name)
	return msg
}

// Greet 被 Hello 调用的函数
func Greet(name string) string {
	return "Hello, " + name + "!"
}

// Calculate 测试多个参数和返回值
func Calculate(a, b int) int {
	result := add(a, b)
	result = sub(result, b)
	result = mul(result, a)
	return result
}

func add(x, y int) int {
	return x + y
}

func sub(x, y int) int {
	return x - y
}

func mul(x, y int) int {
	return x * y
}

// ProcessData 带依赖的复杂函数
func ProcessData(items []string) map[string]int {
	result := make(map[string]int)
	for _, item := range items {
		count := countItem(item)
		result[item] = count
	}
	return result
}

func countItem(item string) int {
	return len(item)
}
