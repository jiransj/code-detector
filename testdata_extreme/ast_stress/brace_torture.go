package ast_stress

// ────────────────────────────────────────────────────────────
// 括号匹配极端混沌压力测试
// 针对 matchBrace 的各种边界条件、字符串/注释混淆、
// 深层嵌套、跨行括号、unicode 全角括号等
// ────────────────────────────────────────────────────────────

import (
	"fmt"
)

// Torture1 JSON 風格深度嵌套在字符串中
func Torture1() string {
	jsonData := `{"level1":{"level2":{"level3":{"level4":{"level5":true}}}}}`
	_ = jsonData
	// 正常大括号在字符串外
	{
		fmt.Println("block 1")
	}
	return "ok"
}

// Torture2 一行内大量字符串拼接含括號
func Torture2() string {
	s := "{" + "}" + "{" + "}" + "(" + ")" + "<" + ">" + "[" + "]"
	s += fmt.Sprintf("{%d}", 42)
	return s
}

// Torture3 字符串和註釋交替出現
func Torture3() int {
	// } { 注释中的括号
	x := 1 /* { 块注释中的括号 } */
	y := 2 // 行注释 { 括号 }
	z := x + y
	{
		/* 又一個 { 塊 } 注釋 */
		return z
	}
}

// Torture4 大量 if-else 嵌套（深度 20+）
func Torture4(n int) int {
	if n <= 0 {
		return 0
	}
	if n == 1 {
		return 1
	}
	if n == 2 {
		return 2
	}
	if n == 3 {
		return 3
	}
	if n == 4 {
		return 4
	}
	if n == 5 {
		return 5
	}
	if n == 6 {
		return 6
	}
	if n == 7 {
		return 7
	}
	if n == 8 {
		return 8
	}
	if n == 9 {
		return 9
	}
	if n == 10 {
		return 10
	}
	if n == 11 {
		return 11
	}
	if n == 12 {
		return 12
	}
	if n == 13 {
		return 13
	}
	if n == 14 {
		return 14
	}
	if n == 15 {
		return 15
	}
	if n == 16 {
		return 16
	}
	if n == 17 {
		return 17
	}
	if n == 18 {
		return 18
	}
	if n == 19 {
		return 19
	}
	return -1
}

// Torture5 Switch 语句大量 case
func Torture5(x int) string {
	switch x {
	case 0:
		return "zero"
	case 1:
		return "one"
	case 2:
		return "two"
	case 3:
		return "three"
	case 4:
		return "four"
	case 5:
		return "five"
	case 6:
		return "six"
	case 7:
		return "seven"
	case 8:
		return "eight"
	case 9:
		return "nine"
	case 10:
		return "ten"
	case 11:
		return "eleven"
	case 12:
		return "twelve"
	case 13:
		return "thirteen"
	case 14:
		return "fourteen"
	case 15:
		return "fifteen"
	default:
		return "many"
	}
}

// Torture6 多個 Select 和 Channel 操作
func Torture6(ch1, ch2 chan int) int {
	select {
	case v := <-ch1:
		select {
		case ch2 <- v:
			return v
		default:
			return -v
		}
	case v := <-ch2:
		select {
		case ch1 <- v:
			return v * 2
		default:
			return -v * 2
		}
	default:
		return 0
	}
}

// Torture7 一行內大量不同括號類型
func Torture7() {
	_ = (1 + 2) * (3 + 4) / (5 + 6) % (7 + 8)
	_ = []int{1, 2, 3, 4, 5}
	_ = map[int]int{1: 10, 2: 20, 3: 30}
	_ = func() int { return 42 }()
	_ = [3]int{1, 2, 3}
}

// Torture8 字符串中含未配對括號
func Torture8() string {
	s1 := "this has { but no matching close in string"
	s2 := "this has } but no open in string"
	s3 := "multiple { braces } in { one } string"
	return s1 + s2 + s3
}

// Torture9 if 內嵌 defer 內嵌 func literal 內嵌 select
func Torture9(items []int) []int {
	result := make([]int, 0, len(items))
	for _, item := range items {
		func(v int) {
			defer fmt.Println("processed", v)
			if v > 0 {
				result = append(result, v)
			}
		}(item)
	}
	return result
}

// Torture10 連續空函數和極簡函數
func Torture10a() {}
func Torture10b() int { return 0 }
func Torture10c(s string) { _ = s }
func Torture10d(a, b int) int { return a + b }
func Torture10e() (x int, y int) { return 1, 2 }

// Torture11 注釋中的各種括號組合
func Torture11() string {
	// /* } { */ /* } { */
	// 下面這行注釋含有各種括號: ( ) [ ] { } < >
	/* 塊注釋: ( ) [ ] { }  " ' ` */
	return "braces in comments"
}

// Torture12 跨多行的複雜表達式括號
func Torture12(a, b, c, d, e, f int) int {
	return (a + b) *
		(c - d) /
		(e * f) +
		((a - b) * (c + d)) -
		((((e - f) * a) + b) / c)
}
