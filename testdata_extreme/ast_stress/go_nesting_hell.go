package ast_stress

// ────────────────────────────────────────────────────────────
// 深度嵌套压力测试
// 目标: 测试 tree-sitter AST 解析器在极端嵌套条件下的表现
// ────────────────────────────────────────────────────────────

func DeepNesting10(n int) int {
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
	return DeepNesting10(n - 1) + DeepNesting10(n - 2)
}

func DeepNesting20(n int) int {
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
	if n == 20 {
		return 20
	}
	return -1
}

// ── 花括号深层嵌套 ─────────────────────────────────

func BraceNesting5() int {
	{
		{
			{
				{
					{
						return 5
					}
				}
			}
		}
	}
}

func BraceNesting10() int {
	{ { { { { { { { { {
		return 10
	} } } } } } } } } }
}

// ── 嵌套花括号 + 字符串中的大括号 ────────────────

func StringBraceMix() string {
	outer := "{"
	inner := func(s string) string {
		return "{" + s + "}"
	}
	result := inner("a") + outer + inner("b") + "}"
	return result
}

// ── 匿名函数内嵌匿名函数 ─────────────────────────

func DeepAnon() func() func() func() int {
	return func() func() func() int {
		return func() func() int {
			return func() int {
				return 42
			}
		}
	}
}

// ── 多层级嵌套的 defer ────────────────────────────

func DeferStack() int {
	a := 0
	defer func() { a++ }()
	defer func() {
		defer func() {
			defer func() {
				a += 4
			}()
			a += 3
		}()
		a += 2
	}()
	a += 1
	return a
}
