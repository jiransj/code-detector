package main

import (
	"encoding/json"
	"fmt"
)

// --- 测试1: 字符串字面量中的伪函数 ---
// 以下字符串中的 "def" 和 "function" 不应被识别为 Go 函数
const fakePythonCode = `
def hello(name):
    print("hello")
def world():
    return 42
`

const fakeJSCode = `function test() { return 1; }
var x = function() { return 2; };`

const fakeJavaCode = `public class Fake { public void method() {} }`

// --- 测试2: 注释中的伪函数定义 ---
/*
func commentFunc1() {
	// 这是一个被注释掉的函数定义，不应被扫描到
	return "hidden"
}
*/

// func commentFunc2() {
// 	fmt.Println("这也是注释中的函数")
// }

// RealFunc1 这是真正的函数
func RealFunc1(x int) int {
	// 字符串中包含函数定义模式
	s := "func fake() { return 1; }"
	_ = s
	return x + 1
}

// --- 测试3: 匿名函数嵌套 ---
func NestedFuncs() func() int {
	// 内部匿名函数
	inner := func() int {
		// 更深层嵌套
		deep := func() int {
			return 42
		}
		return deep() + 1
	}
	return inner
}

// --- 测试4: 方法定义（带接收器）---
type MyStruct struct {
	Value int
}

func (m *MyStruct) MethodA(a, b int) int {
	return m.Value + a + b
}

func (m MyStruct) MethodB() string {
	return fmt.Sprintf("value=%d", m.Value)
}

// --- 测试5: 泛型函数 (Go 1.18+) ---
func GenericFunc[T any, U comparable](items []T, key U) map[T]int {
	result := make(map[T]int)
	for _, item := range items {
		result[item]++
	}
	return result
}

// --- 测试6: 非常长的单行函数 ---
func LongLineFunc() string { a := 1; b := 2; c := 3; d := 4; e := 5; f := 6; return fmt.Sprintf("%d%d%d%d%d%d", a, b, c, d, e, f) }

// --- 测试7: 空函数体 ---
func EmptyFunc() {}

// --- 测试8: JSON 中的函数模式（不应匹配）---
func JSONHandler() string {
	jsonData := `{"func": "test", "function": "demo", "def": "xyz"}`
	return jsonData
}

// --- 测试9: 模板字符串中的函数 ---
func TemplateTest() string {
	// Go 没有模板字符串，但原始字符串中有类似模式
	raw := `function abc() { return 1; }`
	return raw
}

// --- 测试10: 函数名包含数字和下划线 ---
func _privateHelper() int { return 0 }
func Test_123_ABC() string { return "test" }
func dataProcessorV2_0(items []int) []int { return items }

// --- 测试11: JSON 序列化中的函数（字符串内）---
type Config struct {
	Handler string `json:"handler"`
}

func (c *Config) ParseHandler() map[string]interface{} {
	data := `{"handler": "function execute() { return 1; }"}`
	var result map[string]interface{}
	json.Unmarshal([]byte(data), &result)
	return result
}

// --- 测试12: 多个返回值函数 ---
func MultiReturn(a, b int) (int, int, error) {
	if a > b {
		return a, b, nil
	}
	return b, a, fmt.Errorf("swapped")
}

// --- 测试13: 变参函数 ---
func VariadicFunc(items ...int) (sum int) {
	for _, v := range items {
		// sum 是命名返回值
		sum += v
	}
	return
}

// 测试14: init 函数
func init() {
	fmt.Println("init should be detected")
}

// 测试15: 主函数
func main() {
	fmt.Println("Hello")
}
