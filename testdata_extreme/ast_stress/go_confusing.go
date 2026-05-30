package ast_stress

// ────────────────────────────────────────────────────────────
// 混淆 Go 代码压力测试
// 包含模板字符串、注释中含代码、超长行、struct tag 噪声等
// ────────────────────────────────────────────────────────────

import (
	"encoding/json"
	"fmt"
)

// ConfusingFunc1 字符串中包含看起来像代码的内容
func ConfusingFunc1() string {
	// 下面這個字符串裡有 "func() { return }" — 不應被誤解析爲函數
	s1 := "func() { return 42 }"
	// 原始字符串包含程序代碼
	s2 := `package main
func main() {
    fmt.Println("Hello, 世界")
}`
	// 模板字符串（Go 不支持但字符串內容可能有）
	s3 := "template: ${name} = ${value}"
	return s1 + s2 + s3
}

// ConfusingFunc2 包含超長行（>500 字符）來測試邊界情況
func ConfusingFunc2() string {
	longLine := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	return longLine
}

// ConfusingFunc3 struct tag 中包含大量引號和反引號
//go:noinline
func ConfusingFunc3() struct {
	Field1 string `json:"field1" yaml:"field1" xml:"field1,attr"`
	Field2 int    `json:"field2" yaml:"field2" xml:"field2,attr"`
	Field3 bool   `json:"field3,omitempty" yaml:"field3,omitempty"`
} {
	return struct {
		Field1 string `json:"field1" yaml:"field1" xml:"field1,attr"`
		Field2 int    `json:"field2" yaml:"field2" xml:"field2,attr"`
		Field3 bool   `json:"field3,omitempty" yaml:"field3,omitempty"`
	}{
		Field1: "hello",
		Field2: 42,
		Field3: true,
	}
}

// ConfusingFunc4 注釋中包含看起來像函數的東西，且有多個塊注釋嵌套
func ConfusingFunc4(x int) int {
	/* 注釋裡有看起來像函數的內容：
	func fakeFunc() {
	    return "this is in comment"
	}
	*/
	// 另一種：// func alsoFake() { return }
	return x * 2
}

// ConfusingFunc5 包含 URL 等含 // 的內容
func ConfusingFunc5() string {
	url := "https://example.com/path?query=value#fragment"
	_ = url
	// 注釋中的 URL: https://github.com/user/repo/blob/main/file.go#L10-L20
	// data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg"/>
	return "url test"
}

// ConfusingFunc6 使用 fmt.Sprintf 來驗證 AST 解析
func ConfusingFunc6(name string, age int) string {
	// 混合使用不同格式
	a := fmt.Sprintf("Name: %s, Age: %d", name, age)
	b := fmt.Sprintf("Result: %d", age*2)
	c := fmt.Sprintf("%s is %d years old", name, age)
	return a + b + c
}

// ConfusingFunc7 條件編譯標記（Go build tags）
// +build !windows

//go:generate echo "generate directive test"
func ConfusingFunc7() bool {
	return true
}

// ConfusingFunc8 簡體中文注釋 + 日文注釋混合 — 測試 UTF-8 編碼路徑
// この関数は日本語のコメントを含んでいます
// 這個函數包含簡體中文注釋
func ConfusingFunc8() string {
	return "日本語 & 中文"
}

// ConfusingFunc9 字符串內含反斜杠混亂
func ConfusingFunc9() string {
	// 各種轉義序列
	s := "\"Hello\",\n\t'World'\r\n\\path\\to\\file\\"
	// 字符串中嵌入二進制風格內容
	b := "\x00\x01\x02\xFF\xFE"
	return s + b
}

// ConfusingFunc10 非常深的表達式嵌套（非 if 嵌套）
func ConfusingFunc10(a, b, c, d int) int {
	return a + b + c + d +
		(a * b) + (c * d) +
		((a + b) * (c + d)) +
		(((a - b) * (c - d)) + (a * d)) +
		((((a % b) + (c % d)) * a) / (b + 1)) +
		((((a + b) * (c + d)) / ((a - b) + 1)) % 1000)
}
