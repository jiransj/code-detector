package main

import "fmt"

// ============================================================================
// 超级测试: Go 局部/全局函数混杂 + 注释干扰
// 注意: Go 没有真正的嵌套命名函数，但函数字面量（闭包）非常常见
// ============================================================================

// --- 测试1: 全局真实函数 ---
func GlobalFuncA(x int) int {
	return x + 1
}

// --- 测试2: 函数内定义闭包（不应注册为命名函数）---
func ClosureContainer() {
	// 这个匿名函数被赋给变量，不应被注册为独立函数
	add := func(a, b int) int {
		return a + b
	}
	_ = add(1, 2)

	// 另一个闭包
	multiply := func(x, y int) int {
		return x * y
	}
	_ = multiply(3, 4)
}

// --- 测试3: 返回闭包的函数 ---
func MakeAdder(base int) func(int) int {
	// 返回的闭包包含 base 变量
	return func(x int) int {
		return base + x
	}
}

// --- 测试4: 注释中包含完整的函数定义（块注释）---
/*
func BlockCommentedFuncA() string {
	return "this should NOT be detected"
}

func BlockCommentedFuncB(x int) int {
	return x * 2
}
*/

// --- 测试5: 注释中包含部分函数定义（行注释）---
// func LineCommentedFuncA() string {
// 	return "this should NOT be detected"
// }

// func LineCommentedFuncB(x int) int {
// 	return x * 100
// }

// --- 测试6: 字符串中嵌入完整函数（不应提取）---
func StringEmbeddedFuncs() {
	// Go 原始字符串中的函数定义
	goCode := `
package calculator

func Add(a, b int) int {
	return a + b
}

func Subtract(a, b int) int {
	return a - b
}

func Multiply(a, b int) int {
	return a * b
}
`
	_ = goCode

	// 双引号转义字符串中的函数
	jsonData := "{\"func\": \"handler\", \"code\": \"function execute() { return true; }\"}"
	_ = jsonData
}

// --- 测试7: 真实函数中有与注释中同名的函数 ---
func ConfigLoader() string {
	return "loaded"
}

/* 同名函数在注释中不应冲突
func ConfigLoader() string {
	return "commented"
}
*/

// --- 测试8: 多层注释嵌套陷阱 ---
func RealFuncInNestedComments() string {
	/*
	 * func deeplyNestedCommented() {
	 *     // 这里即使有 func 也不应被检测
	 *     func insideBlock() {}
	 * }
	 */
	return "real"
}

// --- 测试9: if 内部的函数定义（Go 不支持，但测试 parser 鲁棒性）---
func ConditionalScope() {
	if true {
		// 这不是函数定义，是闭包赋值
		fn := func() int { return 1 }
		_ = fn
	}
}

// --- 测试10: 方法接收器与全局函数混杂 ---
type Calculator struct {
	base int
}

func (c *Calculator) Add(x int) int {
	return c.base + x
}

func (c *Calculator) Subtract(x int) int {
	return c.base - x
}

// 普通全局函数，与方法同名但参数不同
func Add(x, y int) int {
	return x + y
}

func Subtract(x, y int) int {
	return x - y
}

// --- 测试11: init 函数与普通函数混合 ---
func init() {
	fmt.Println("init 1")
}

func init() {
	fmt.Println("init 2")
}

// --- 测试12: 函数内有大量字符串，包含各种语言函数定义 ---
func MultiLangStrings() {
	pyCode := `
def python_func_a():
    return "a"

def python_func_b():
    return "b"

class PythonClass:
    def method(self):
        pass
`
	jsCode := `
function jsFuncA() {
    return "a";
}
const jsFuncB = () => {
    return "b";
};
class JSClass {
    method() {
        return "c";
    }
}
`
	javaCode := `
public class JavaClass {
    public void javaMethod() {
        System.out.println("hello");
    }
}
`
	_ = pyCode
	_ = jsCode
	_ = javaCode
}

// --- 测试13: 同文件大量函数，模拟真实项目 ---
func AuthLogin(username, password string) bool {
	return validateCredentials(username, password)
}

func validateCredentials(user, pass string) bool {
	return user == "admin" && pass == "secret"
}

func AuthLogout(sessionID string) {
	clearSession(sessionID)
}

func clearSession(id string) {
	_ = id
}

func GetUserProfile(uid int) string {
	return fetchProfile(uid)
}

func fetchProfile(uid int) string {
	return fmt.Sprintf("profile_%d", uid)
}

// --- 测试14: 空函数 ---
func Nop() {}

func Nop2() {}

// --- 测试15: defer 中的闭包（不应注册）---
func DeferClosure() {
	defer func() {
		fmt.Println("deferred")
	}()

	defer func(msg string) {
		fmt.Println(msg)
	}("hello")
}

// --- 测试16: goroutine 中的闭包 ---
func GoRoutineClosure() {
	go func() {
		fmt.Println("goroutine")
	}()

	go func(msg string) {
		fmt.Println(msg)
	}("async")
}

// --- 测试17: 非常深的多层闭包嵌套 ---
func DeepClosureNesting() func() func() int {
	a := func() func() int {
		b := func() int {
			c := func() int {
				return 42
			}
			return c()
		}
		return b
	}
	return a
}

// --- 测试18: 泛型函数内部的局部位函数 ---
func GenericProcessor[T any](items []T) []T {
	// 闭包操作
	process := func(item T) T {
		return item
	}
	result := make([]T, len(items))
	for i, item := range items {
		result[i] = process(item)
	}
	return result
}

// --- 测试19: 注释中有看起来像真实代码的块 ---
/* 下面的代码块看起来像真实项目代码，但全部在注释中
package handler

import "net/http"

func HandleRequest(w http.ResponseWriter, r *http.Request) {
    // 处理请求
    data := fetchData(r.URL.Query().Get("id"))
    w.Write(data)
}

func fetchData(id string) []byte {
    return []byte("data for " + id)
}

func validateInput(input string) bool {
    return len(input) > 0
}
*/

// --- 测试20: 真实的最终函数 ---
func UltimateRealFunc() string {
	return "I am real"
}
