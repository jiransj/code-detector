package ast_stress

// ────────────────────────────────────────────────────────────
// 语法噪声压力测试
// 包含空行/空白字符变体/混合缩进等
// ────────────────────────────────────────────────────────────

import (
	"fmt"
)

// Noise1 函数之間大量空行和空白
func Noise1() int { return 1 }





// 7 行空行







func Noise2() int { return 2 }













func Noise3() int { return 3 }

// Noise4 混合製表符和空格縮進
func Noise4() int {
			// 製表符縮進
			    // 空格縮進
		 		// 混合製表符+空格
	return 4
}

// Noise5 行尾多餘空格和分號
func Noise5() int {	
	return 5	
}   

// Noise6 多個表達式在一行
func Noise6(a, b int) (int, int) { x := a + b; y := a - b; return x, y }

// Noise7 空白標識符大量使用
func Noise7() (int, string) {
	_ = "ignored"
	_, _ = fmt.Println("hello")
	a, _ := fmt.Printf("test"), fmt.Sprintf("ignored")
	_ = a
	return 42, "ok"
}

// Noise8 不同的換行風格在一個文件中
// 使用 \n（LF）正常換行；理論上文件可能混合 CRLF 和 LF
func Noise8() bool {
	if true {
		return true
	}
	return false
}

// Noise9 在標識符和運算符之間有大量空格
func Noise9(   a   int   ,   b   int   )      int    {
	if     a     >     b     {
		return     a
	}
	return     b
}

// Noise10 函數體爲空、nil 返回、省略類型等
func Noise10()                {}
func Noise11()                { return }
func Noise12(a, b, c int)     { _ = a + b + c }
