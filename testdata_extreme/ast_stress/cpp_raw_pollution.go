package ast_stress

// ────────────────────────────────────────────────────────────
// C++ R" 字符串污染攻击向量测试
// matchBrace() 中的 ccR 分支在没有 C++ 的语言中可能
// 错误触发原始字符串跳过逻辑，导致括号匹配偏移。
// 这些测试专门验证 Go 解析器不被该逻辑污染。
// ────────────────────────────────────────────────────────────

import (
	"fmt"
)

// RPollution1 测试 R" 在字符串中和注释中不影响括号匹配
func RPollution1() string {
	// 注释中有 R"test" 不影響解析
	s := `R"(content)delimiter"`
	_ = s
	return "ok" // 行尾注释 R"test"
}

// RPollution2 变量 R 紧邻字符串 "(" 的表达式
func RPollution2() int {
	R := "prefix"
	_ = R + "("
	x := 1
	y := 2
	if x < y {
		return x + y
	}
	return x - y
}

// RPollution3 R"delim(content)delim" 模式在字符串中
func RPollution3() string {
	x := `R"delimiter(start`
	y := `end)delimiter"`
	_ = x
	_ = y
	if true {
		return "nested brace { inside if }"
	}
	return "outside"
}

// RPollution4 raw string 中含有 R" 模式
func RPollution4(items []string) []string {
	result := make([]string, 0, len(items))
	for i, item := range items {
		r := fmt.Sprintf(`R"delim-%d(%s)delim-%d"`, i, item, i)
		result = append(result, r)
	}
	return result
}

// RPollution5 混合 { } 和 R" 模式字符串在同一个作用域
func RPollution5(a, b int) int {
	if a > 0 {
		fmt.Println("positive")
	}
	r1 := `R"(test)"`
	_ = r1
	switch b {
	case 1:
		return a + b
	case 2:
		if a > 10 {
			return a * b
		}
		return a - b
	default:
		return 0
	}
}

// RPollution6 变量名以 R 开头但正常代码
func RPollution6() string {
	Result := "this starts with R but is followed by e, not quote"
	Ready := "also starts with R"
	_ = Ready
	return Result
}

// RPollution7 R"R(R)R" 模式在字符串中
func RPollution7() string {
	commentR := `R"R(R)R"`
	_ = commentR
	arr := []string{"a", "b", "c"}
	for _, v := range arr {
		_ = fmt.Sprintf("R%s", v)
	}
	return "R-pollution-test"
}

// RPollution8 R" 模式在复杂 for 循环前后
func RPollution8(n int) int {
	result := 0
	str := "start" + `R"(` + "end"
	_ = str
	for i := 0; i < n; i++ {
		for j := 0; j < i; j++ {
			result += i*j + 1
		}
	}
	return result
}

// RPollution9 R" 在 defer 或 go 语句后
func RPollution9() {
	defer fmt.Println("cleanup")
	r := `R"delim(content)delim"`
	_ = r
	go fmt.Println("async")
}

// RPollution10 极度混亂的 R" 模式 + 嵌套控制流
func RPollution10(values []int) (result []int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	s1 := `R"("`
	s2 := `R"(nested)"`
	s3 := `R"delim(data)delim"`
	_, _, _ = s1, s2, s3

	result = make([]int, 0, len(values))
	for _, v := range values {
		if v < 0 {
			return nil, fmt.Errorf("negative: %d", v)
		}
		result = append(result, v*v)
	}
	return result, nil
}
