package ast_stress

// ────────────────────────────────────────────────────────────
// Unicode/特殊字符混淆压力测试
// ────────────────────────────────────────────────────────────

import "fmt"

// Unicode1 中文函數名（Go 1.26 支持 Unicode 標識符）
func Unicode1中文関数名() string {
	return "unicode identifier"
}

// Unicode2 希臘字母變量名
func Unicode2() string {
	α, β := "alpha", "beta"
	γ := fmt.Sprintf("%s-%s", α, β)
	return γ
}

// Unicode3 Emoji 在注釋和字符串中
func Unicode3() string {
	// 🚀 火箭 emoji 在注釋中
	// 🔥 火焰 emoji
	// ❤️ 愛心
	s := "emoji in string: 🎉🎊🎈"
	// 含 emoji 的字符串: "😀😎🤖"
	t := `emoji in raw: 💻🖥️📱`
	return s + t
}

// Unicode4 全角字符（混淆視覺）
func Unicode4() string {
	// 全角括号： （） 不應被當作ASCII括號
	// 全角花括號： ｛｝ 不應被當作 {}
	// 全角冒號： ：
	s := "fullwidth: （test） ｛test｝ ："
	return s
}

// Unicode5 零寬字符（潛在攻擊向量）
func Unicode5() string {
	// 零寬空格 (U+200B) 和零寬連字 (U+200D)
	// Go 編譯器會忽略，但解析器可能被混淆
	s := "zero\u200Bwidth\u200Dspace"
	return s
}

// Unicode6 雙向文本覆蓋（潛在的 Trojan Source 攻擊）
func Unicode6() string {
	// 以下包含 Unicode 雙向覆蓋字符，用於測試解析器是否會混淆
	// U+202E RIGHT-TO-LEFT OVERRIDE
	s := "RLO\u202Etest"
	// U+202D LEFT-TO-RIGHT OVERRIDE
	t := "LRO\u202Dtest"
	return s + t
}

// Unicode7 上標/下標數字
func Unicode7() string {
	s := "superscript: ¹²³⁴⁵⁶⁷⁸⁹⁰"
	t := "subscript: ₁₂₃₄₅₆₇₈₉₀"
	return s + t
}

// Unicode8 各國文字混排
func Unicode8() string {
	a := "English"
	b := "中文"
	c := "日本語"
	d := "한국어"
	e := "Русский"
	f := "العربية"
	g := "हिन्दी"
	return fmt.Sprintf("%s/%s/%s/%s/%s/%s/%s", a, b, c, d, e, f, g)
}

// Unicode9 數學符號作爲變量（Go 不允許但字符串中可以）
func Unicode9() string {
	// 這些都在字符串中，不影響編譯
	s := "∀x ∈ ℝ: x² ≥ 0"
	t := "∑_{i=1}^{n} i = n(n+1)/2"
	return s + t
}

// Unicode10 模擬同形字攻擊（homoglyph attack）
// 使用看起來像 "main" 但實際是不同 Unicode 碼點的名稱
func Unicode10() string {
	// Cyrillic 'а' (U+0430) vs Latin 'a' (U+0061)
	// 這兩個看起來一樣但不同
	latinA := "a"     // U+0061
	cyrillicA := "а"  // U+0430
	return fmt.Sprintf("%s != %s", latinA, cyrillicA)
}
