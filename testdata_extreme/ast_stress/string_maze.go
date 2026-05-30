package ast_stress

// ────────────────────────────────────────────────────────────
// 字符串迷宫压力测试
// 包含各种字符串边界条件、嵌套引号、转义字符
// ────────────────────────────────────────────────────────────

import "fmt"

// Str1 字符串中包含所有三種引號類型
func Str1() string {
	a := "double: \"inner\""
	b := `raw: "double" and ` + "`" + "backtick" + "`" + ``
	return fmt.Sprintf("%s | %s", a, b)
}

// Str2 大量轉義序列
func Str2() string {
	s := "\n\t\r\\\"\'\x00\x01\x02\x03\xFF"
	return fmt.Sprintf("%q", s)
}

// Str3 原始字符串包含引號混亂
func Str3() string {
	a := `plain raw string`
	b := `raw with "double quotes"`
	c := `raw with ` + "`" + `backtick` + "`" + ` inside`
	return a + b + c
}

// Str4 字符串中包含代碼生成的混亂內容
func Str4() string {
	// 模擬代碼生成器的輸出
	s := "func(a, b int) (int, error) { " +
		"if a > b { return a, nil } " +
		"return b, fmt.Errorf(\"a (%d) <= b (%d)\", a, b) " +
		"}"
	return s
}

// Str5 字符串和變量聲明混合在複雜表達式中
func Str5(items []string) map[string]string {
	result := make(map[string]string, len(items))
	for i, item := range items {
		key := fmt.Sprintf("key_%d", i)
		val := fmt.Sprintf("value_%s", item)
		result[key] = val
	}
	return result
}

// Str6 字符串中嵌入 JSON/XML/YAML/TOML
func Str6() string {
	json := `{"name":"test","data":{"values":[1,2,3],"nested":{"active":true}}}`
	xml := `<root><item id="1"><name>test</name></item></root>`
	yaml := "key: value\n  nested:\n    - item1\n    - item2"
	toml := "[section]\nkey = \"value\"\narr = [1, 2, 3]"
	return json + xml + yaml + toml
}

// Str7 字符串中包含路徑分隔符（類似 C++ R 字符串的模式）
func Str7() string {
	// Windows 路徑: C:\Users\name\Documents\"file".txt
	path1 := `C:\Users\name\Documents\file.txt`
	path2 := "C:\\Users\\name\\Documents\\\"file\".txt"
	// 類似 C++ R" 的模式在字符串中
	likeCpp := "R\"delim(content)delim\""
	// 另一個變體
	likeCpp2 := "R\"("
	_ = likeCpp
	_ = likeCpp2
	return path1 + path2
}

// Str8 混合長字符串和短字符串
func Str8() string {
	s1 := "a"
	s2 := "bb"
	s3 := "ccc"
	s4 := "dddd"
	s5 := "eeeee"
	s6 := "ffffff"
	s7 := "ggggggg"
	s8 := "hhhhhhhh"
	s9 := "iiiiiiiii"
	s10 := "jjjjjjjjjj"
	return s1 + s2 + s3 + s4 + s5 + s6 + s7 + s8 + s9 + s10
}

// Str9 交替使用 fmt.Sprintf 和字符串拼接
func Str9(name string, count int) string {
	a := fmt.Sprintf("user-%s", name)
	b := fmt.Sprintf("count-%d", count)
	c := fmt.Sprintf("%s-%s", a, b)
	d := "final-" + c
	return d
}

// Str10 原始字符串包含 Go 代碼（模板生成場景）
func Str10() string {
	template := `package main

func main() {
    fmt.Println("Hello, World!")
    // 可以在這裏插入 {{ .Name }}
}`
	return template
}
