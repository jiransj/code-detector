package ast_stress

// ────────────────────────────────────────────────────────────
// 花括号匹配混乱压力测试
// 专门测试 matchBrace 在字符串/注释/各种边界中的表现
// ────────────────────────────────────────────────────────────

import (
	"fmt"
)

// Brace1 字符串內有大量 {}，不應被誤匹配
func Brace1() string {
	// JSON 字符串: {"users":[{"id":1,"name":"a"},{"id":2,"name":"b"}]}
	s := `{"users":[{"id":1,"name":"a"},{"id":2,"name":"b"}]}`
	return s
}

// Brace2 包含 C++ R" 字符串模式的 Go 代碼（測試 matchBrace 污染 bug）
func Brace2() string {
	// Go 中 R 沒有特殊含義，但 matchBrace 的 ccR 分支可能錯誤觸發
	// 下面這種模式不應該觸發 C++ 原始字符串跳過邏輯
	rest1 := "R\"delimiter(content)delimiter\""
	_ = rest1

	// 更接近 C++ R" 的模式
	rest2 := "R\"("
	_ = rest2
	rest3 := ")"
	_ = rest3
	rest4 := ")delim\""
	_ = rest4

	// 正常 Go 代碼 - 如果 matchBrace 被錯誤跳過，下面的括號匹配會錯
	if true {
		fmt.Println("this should still parse correctly")
	}
	return "ok"
}

// Brace3 原始字符串中包含大括號 + 反引號混合
func Brace3() string {
	a := `{ "key": "value" }`
	b := "{ \"key\": \"value\" }"
	c := ` ` + "`" + `backtick inside string` + "`" + ` `
	return a + b + c
}

// Brace4 字符串內有未閉合括號（但字符串是完整的）
func Brace4() string {
	s := "unclosed { bracket in string"
	_ = s
	s2 := "closing } bracket only"
	return s2
}

// Brace5 大量嵌套括號在一個表達式內
func Brace5(items []int) map[string]map[string]int {
	result := make(map[string]map[string]int)
	for _, item := range items {
		key := fmt.Sprintf("item-%d", item)
		if _, ok := result[key]; !ok {
			result[key] = make(map[string]int)
		}
		result[key][fmt.Sprintf("sub-%d", item%3)] = item * 2
	}
	return result
}

// Brace6 注釋中的大括號（不應被計數）
func Brace6() int {
	// {{{
	/* }}} */
	// 括號平衡: { } { } { }
	return 42
}

// Brace7 一行內極大量 {}
func Brace7() int {
	{ { { { { { { { { { { { { { { { { { { {
		{ { { { { { { { { { { { { { { { { { { {
			return 42
		} } } } } } } } } } } } } } } } } } } }
	} } } } } } } } } } } } } } } } } } } }
}

// Brace8 字符串中包含 Go 代碼（json 序列化場景）
func Brace8() string {
	data := struct {
		Code string `json:"code"`
	}{Code: "func(x int) int { return x * 2 }"}
	raw, _ := fmt.Sprintf("%v", data)
	return raw
}

// Brace9 混合使用所有三種引號
func Brace9() string {
	a := "double \"quoted\" string with {braces}"
	b := 's'
	c := `raw string with "quotes" and {braces} and ` + "`backticks`" + ``
	_ = a
	_ = b
	return c
}

// Brace10 轉義引號在字符串中
func Brace10() string {
	// \" 和 \' 不應該結束字符串
	s := "He said: \"Hello {world}\" and she said: \"Hi {there}\""
	_ = s
	t := "It\'s {fine} isn\'t it?"
	return t
}
