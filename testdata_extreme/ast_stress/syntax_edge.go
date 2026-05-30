package ast_stress

// ────────────────────────────────────────────────────────────
// Go 语法边界条件压力测试
// 针对 tree-sitter 对 Go 高级语法的解析能力
// ────────────────────────────────────────────────────────────

import (
	"fmt"
	"io"
)

// ── 类型定义 ───────────────────────────────────────

type MyInt int
type MyString string
type HandlerFunc func(string) error

type ReaderWriter interface {
	io.Reader
	io.Writer
	Close() error
}

type Embedded struct {
	io.Reader
	io.Writer
	name string
}

// Syntax1 类型 switch
func Syntax1(v interface{}) string {
	switch v := v.(type) {
	case nil:
		return "nil"
	case int:
		return fmt.Sprintf("int:%d", v)
	case string:
		return fmt.Sprintf("string:%q", v)
	case bool:
		return fmt.Sprintf("bool:%v", v)
	case []byte:
		return fmt.Sprintf("bytes:%d", len(v))
	case error:
		return v.Error()
	case interface{ String() string }:
		return v.String()
	default:
		return fmt.Sprintf("unknown:%T", v)
	}
}

// Syntax2 结构体嵌入 + 方法提升
func (e *Embedded) Read(p []byte) (int, error) {
	return e.Reader.Read(p)
}

func (e *Embedded) Write(p []byte) (int, error) {
	return e.Writer.Write(p)
}

func (e *Embedded) Close() error {
	return nil
}

// Syntax3 iota 常量
const (
	SyntaxA = iota
	SyntaxB
	SyntaxC
	SyntaxD = iota + 10
	SyntaxE
	SyntaxF = iota * 2
)

// Syntax4 位运算和复杂常量表达式
const (
	FlagNone  = 0
	FlagRead  = 1 << 0
	FlagWrite = 1 << 1
	FlagExec  = 1 << 2
	FlagAll   = FlagRead | FlagWrite | FlagExec
	Mask      = ^FlagAll
)

// Syntax5 通道方向
func Syntax5(out chan<- string, in <-chan string) {
	go func() {
		for msg := range in {
			out <- msg
		}
		close(out)
	}()
}

// Syntax6 defer 链式调用
func Syntax6() (err error) {
	f := func() error { return nil }
	defer f()
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return nil
}

// Syntax7 init 函数
func init() {
	fmt.Println("syntax test init")
}

// Syntax8 逗号 OK 惯用法
func Syntax8(m map[string]int, key string) int {
	if v, ok := m[key]; ok {
		return v
	}
	return -1
}

// Syntax9 标签和 goto
func Syntax9(items []int) int {
	target := 0
	if len(items) == 0 {
		goto end
	}
	for i, v := range items {
		if v < 0 {
			break
		}
		target = i
		_ = v
	}
end:
	return target
}

// Syntax10 recover 在 defer 外
func Syntax10() {
	defer func() {
		recover()
	}()
	panic("test")
}
