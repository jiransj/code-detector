package ast_stress

// ────────────────────────────────────────────────────────────
// 单函数大量调用压力测试
// 目标: 测试 tree-sitter AST 解析器在一个函数内
//       处理大量 call_expression 的性能与正确性
// ────────────────────────────────────────────────────────────

import "fmt"

func helper0(n int) int { return n + 0 }
func helper1(n int) int { return n + 1 }
func helper2(n int) int { return n + 2 }
func helper3(n int) int { return n + 3 }
func helper4(n int) int { return n + 4 }
func helper5(n int) int { return n + 5 }
func helper6(n int) int { return n + 6 }
func helper7(n int) int { return n + 7 }
func helper8(n int) int { return n + 8 }
func helper9(n int) int { return n + 9 }

// MassCall 包含 50 个函数调用的单一函数
// 用于测试 AST 解析器在大调用密度场景下的准确性
func MassCall(x int) int {
	a := helper0(x)
	a += helper1(x)
	a += helper2(x)
	a += helper3(x)
	a += helper4(x)
	a += helper5(x)
	a += helper6(x)
	a += helper7(x)
	a += helper8(x)
	a += helper9(x)
	_ = fmt.Sprintf("value: %d", a)
	_ = fmt.Sprintf("double: %d", a*2)
	_ = fmt.Sprintf("triple: %d", a*3)
	_ = fmt.Sprintf("quad: %d", a*4)
	_ = fmt.Sprintf("quint: %d", a*5)
	_ = fmt.Sprintf("sext: %d", a*6)
	_ = fmt.Sprintf("sept: %d", a*7)
	_ = fmt.Sprintf("oct: %d", a*8)
	_ = fmt.Sprintf("non: %d", a*9)
	_ = fmt.Sprintf("dec: %d", a*10)
	_ = fmt.Sprintf("sum: %d", a+0)
	_ = fmt.Sprintf("sum: %d", a+1)
	_ = fmt.Sprintf("sum: %d", a+2)
	_ = fmt.Sprintf("sum: %d", a+3)
	_ = fmt.Sprintf("sum: %d", a+4)
	_ = fmt.Sprintf("sum: %d", a+5)
	_ = fmt.Sprintf("sum: %d", a+6)
	_ = fmt.Sprintf("sum: %d", a+7)
	_ = fmt.Sprintf("sum: %d", a+8)
	_ = fmt.Sprintf("sum: %d", a+9)
	return a +
		helper0(a) +
		helper1(a) +
		helper2(a) +
		helper3(a) +
		helper4(a) +
		helper5(a) +
		helper6(a) +
		helper7(a) +
		helper8(a) +
		helper9(a) +
		helper0(x) +
		helper1(x) +
		helper2(x) +
		helper3(x) +
		helper4(x) +
		helper5(x) +
		helper6(x) +
		helper7(x) +
		helper8(x) +
		helper9(x)
}
