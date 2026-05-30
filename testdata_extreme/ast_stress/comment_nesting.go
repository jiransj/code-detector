package ast_stress

// ────────────────────────────────────────────────────────────
// 注释嵌套混乱压力测试
// ────────────────────────────────────────────────────────────

import "fmt"

// Comment1 块注释中包含看起来像块注释的内容
// /* 这看起来像是块注释的开始，但实际上在行注释中 */
func Comment1() int {
	/*
		这里有一个单独的 /* 应该不算嵌套块注释 */
		Go 不支持嵌套块注释，所以第一个 */ 就结束注释了
		下面是正常代码
	*/
	/* 这也算 */ return 42
}

// Comment2 行注释末尾没有换行（文件结尾）
// 文件应正确处理注释到 EOF 的情况
func Comment2() string {
	return "no newline after this comment"
}

// Comment3 大量前缀 // 的行
func Comment3() string {
	/////////////////////////////////////////////////
	// 这一行是注释 // 这是注释内的注释 //
	/////////////////////////////////////////////////
	return "comment chaos"
}

// Comment4 注释中 URL 含 // 不应导致注释提前结束
func Comment4() string {
	// 参见 https://example.com/path?query=value#section 获取更多信息
	// 另一个 URL: http://localhost:8080/api/v1/users?page=1&limit=10
	// 数据源: file:///C:/path/to/file.txt
	// data:application/json;base64,e30=
	return "urls checked"
}

// Comment5 混合的块注释和行注释边界
func Comment5() int {
	/* 块注释开始 */ // 行注释开始
	/* 另一个块注释 */ x := 42
	// 行注释 /* 块注释开始但在行注释中 */
	y := x + 1
	/* 多行
	   块注释
	   跨三行 */ z := y * 2
	return z
}

// Comment6 注释和代码在同一行，注释包含括号
func Comment6() int {
	return 42 // 最后的注释 { return 42 } /* block */ // line
}

// Comment7 块注释包含多行 Go 代码
func Comment7() string {
	/*
	func commentedOut() string {
	    return "this function is commented out"
	}
	*/
	return "active code"
}

// Comment8 注释紧挨着函数签名
// 函数名前有大量注释，返回值后有行注释
func Comment8() string { // 返回值后的行注释
	return "comment edge"
}

// Comment9 注释和 fmt 调用混合
func Comment9() {
	// print
	fmt.Println("a")
	/* block */ fmt.Println("b")
	// comment
	// another
	fmt.Println("c")
}

// Comment10 行尾多个不同的块注释
func Comment10() int {
	return 1 /* first */ + 2 /* second */ + 3 /* third */
}
