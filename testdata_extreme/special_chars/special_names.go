package main

import "fmt"

// === 极限测试: 特殊字符干扰 ===

// --- 测试1: 表情符号在注释中 ---

// RealFuncWithEmoji 这是一个带 😀 emoji 🚀 的注释
func RealFuncWithEmoji() string {
	// 这里也有 emoji: ⭐🔥✅🎯
	s := "字符串中的emoji: 😊🌍"
	return s
}

// --- 测试2: 零宽字符 ---

// ZeroWidthTest 零宽字符测试
func ZeroWidthTest() string {
	// 以下注释包含零宽空格 (U+200B) 和零宽非连接符 (U+200C)
	// 这些不应该影响函数检测
	s := "test\u200Bstring\u200Cwith\u200Dzeros"
	return s
}

// --- 测试3: RTL 覆盖字符 ---

// RTLTest RTL 覆盖字符测试
func RTLTest() string {
	// \u202E 是 RIGHT-TO-LEFT OVERRIDE
	s := "hello\u202Eworld"
	return s
}

// --- 测试4: 中文函数名（Go 不支持，但中文字符可能出现在注释/字符串中）---
func ChineseCommentTest() string {
	// 计算总计：这是一个中文注释
	// 获取用户名：也是中文注释
	return "测试"
}

// --- 测试5: 全角字符在字符串中 ---
func FullWidthTest() string {
	s := "ｆｕｎｃ" // 全角字符
	_ = s
	// 全角括号不应匹配: （ ）
	return "test"
}

// --- 测试6: 数学符号在注释中 ---
func MathSymbolTest() int {
	// 数学符号: ∑ ∫ π ≈ ≠ ≤ ≥ ∞
	// 希腊字母: α β γ δ ε θ λ μ
	return 42
}

// --- 测试7: 控制字符在字符串中 ---
func ControlCharTest() map[string]string {
	return map[string]string{
		"tab":    "\t",
		"null":   "\x00",
		"bell":   "\a",
		"escape": "\x1b",
	}
}

// --- 测试8: 字符串字面量中有函数定义模式 ---
func StringWithFuncPattern() {
	// 以下字符串中的 "func" 不应触发匹配
	s1 := "func helper() { return 1; }"
	s2 := `func anotherHelper() { return 2; }`
	s3 := "func(a, b int) int { return a + b }" // 匿名函数类型
	_, _, _ = s1, s2, s3
}

// --- 测试9: 非常长的函数名 ---
func ThisIsAnExtremelyLongFunctionNameThatShouldStillBeCorrectlyDetectedByTheScannerAndStoredInTheDatabaseWithoutAnyIssues(x int) int {
	return x
}

// --- 测试10: 函数名包含特殊 Go 标识符字符 ---
func _init() int    { return 0 }  // 私有函数
func init_test() int { return 0 } // 以下划线开头

// --- 测试11: 裸函数（无主体）---
// 以下只有声明，不应包含
// func declaredOnly()

// --- 测试12: 函数体包含中文字符串 ---
func ChineseStringFunc() string {
	return "这是一个函数返回的中文字符串：你好，世界！"
}

// --- 测试13: 退格字符干扰 ---
func BackspaceTest() string {
	// \b 退格字符
	return "abc\bdef"
}

// --- 测试14: 空字节在字符串中 ---
func NullByteInString() string {
	return "null\x00byte"
}

// --- 测试15: 多个连续空函数 ---
func empty1() {}
func empty2() {}
func empty3() {}

// --- 测试16: 函数体包含原始字符串字面量中的大括号 ---
func RawStringWithBraces() string {
	// 原始字符串中的 {} 不应干扰括号匹配
	s := `{ "key": "value", "nested": {"a": 1} }`
	return s
}

// --- 测试17: 函数中有很多嵌套的大括号 ---
func ManyBrackets(items []int) map[int]int {
	result := make(map[int]int)
	for _, item := range items {
		if item > 0 {
			result[item] = item * item
		} else {
			result[item] = 0 - item
		}
	}
	return result
}

// --- 测试18: 字符串中的函数定义被正确忽略 ---
func IgnoreStrings() {
	// 多行原始字符串中的函数定义
	json := `{
		"func": "handler",
		"code": "function test() { return 1; }"
	}`
	_ = json

	// 解释型字符串中的函数定义
	code := "package main\nfunc fake() {\n\tfmt.Println(\"fake\")\n}"
	_ = code
}

// 真正的函数
func realExtremeGoFunc() int {
	return 42
}
