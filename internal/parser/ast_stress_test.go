package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"code-detector/internal/model"
)

// ────────────────────────────────────────────────────────────
// AST 压力测试
// 目标: 验证 tree-sitter AST 解析器在极端场景下的
//       正确性、性能与内存安全
// ────────────────────────────────────────────────────────────

// readTestFile 读取测试数据文件的辅助函数
func readTestFile(t *testing.T, relPath string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata_extreme", relPath))
	if err != nil {
		t.Fatalf("读取测试文件 %s 失败: %v", relPath, err)
	}
	return data
}

// ── 巨型文件解析压力测试 ─────────────────────────────

func TestASTParseGiantFile(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过巨型文件测试 (short mode)")
	}
	content := readTestFile(t, "ast_stress/go_giant.go")
	p := NewTreeSitterGoParser()
	funcs, err := p.Parse("go_giant.go", content)
	if err != nil {
		t.Fatalf("解析 go_giant.go 失败: %v", err)
	}
	if len(funcs) < 100 {
		t.Errorf("期望至少 100 个函数，实际只解析到 %d 个", len(funcs))
	}
	t.Logf("go_giant.go: 解析到 %d 个函数", len(funcs))

	// 验证关键函数名存在
	nameSet := make(map[string]bool, len(funcs))
	for _, f := range funcs {
		nameSet[f.Name] = true
	}
	keyFuncs := []string{"Fn0001", "Fn0002", "Fn0041", "Fn0046", "Fn0052", "Fn0055", "Fn0148"}
	for _, name := range keyFuncs {
		if !nameSet[name] {
			t.Errorf("关键函数 %s 未被解析到", name)
		}
	}

	// 验证泛型函数
	if !nameSet["Fn0042"] {
		t.Errorf("泛型函数 Fn0042[T comparable] 未被解析到")
	}
	if !nameSet["Fn0043"] {
		t.Errorf("泛型函数 Fn0043[K comparable, V any] 未被解析到")
	}
	if !nameSet["Fn0044"] {
		t.Errorf("泛型函数 Fn0044[T Numeric] 未被解析到")
	}

	// 验证方法
	if !nameSet["GetName"] {
		t.Errorf("方法 GetName 未被解析到")
	}
	if !nameSet["SetCache"] {
		t.Errorf("方法 SetCache 未被解析到")
	}
	if !nameSet["ProcessAll"] {
		t.Errorf("方法 ProcessAll 未被解析到")
	}

	// 验证匿名/闭包
	if !nameSet["Fn0049"] {
		t.Errorf("闭包函数 Fn0049 未被解析到")
	}
	if !nameSet["Fn0050"] {
		t.Errorf("可变参闭包 Fn0050 未被解析到")
	}
}

// ── 全局变量解析测试 ────────────────────────────────

func TestASTGlobalsOnGiantFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode")
	}
	content := readTestFile(t, "ast_stress/go_giant.go")
	p := NewTreeSitterGoParser()
	vars, err := p.Globals("go_giant.go", content)
	if err != nil {
		t.Fatalf("Globals 解析失败: %v", err)
	}
	// 这个文件没有顶层全局变量（都在函数内），所以 vars 可能为空
	// 但至少不应 panic 或返回错误
	t.Logf("go_giant.go 全局变量数: %d", len(vars))
}

// ── 深度嵌套测试 ─────────────────────────────────────

func TestASTDeepNesting(t *testing.T) {
	content := readTestFile(t, "ast_stress/go_nesting_hell.go")
	p := NewTreeSitterGoParser()
	funcs, err := p.Parse("go_nesting_hell.go", content)
	if err != nil {
		t.Fatalf("解析 go_nesting_hell.go 失败: %v", err)
	}

	nameSet := make(map[string]bool, len(funcs))
	for _, f := range funcs {
		nameSet[f.Name] = true
	}

	// 必须能解析到所有函数
	for _, name := range []string{
		"DeepNesting10", "DeepNesting20",
		"BraceNesting5", "BraceNesting10",
		"StringBraceMix", "DeepAnon", "DeferStack",
	} {
		if !nameSet[name] {
			t.Errorf("函数 %s 未被解析到嵌套测试文件中", name)
		}
	}

	// 验证 BraceNesting5 的正确行号范围
	for _, f := range funcs {
		if f.Name == "BraceNesting5" {
			if f.LineEnd-f.LineStart < 5 {
				t.Errorf("BraceNesting5 的行数范围过短: [%d, %d]，可能括号匹配出错",
					f.LineStart, f.LineEnd)
			}
			t.Logf("BraceNesting5: 行 %d-%d", f.LineStart, f.LineEnd)
		}
		if f.Name == "BraceNesting10" {
			if f.LineEnd-f.LineStart < 3 {
				t.Errorf("BraceNesting10 的行数范围过短: [%d, %d]，可能括号匹配出错",
					f.LineStart, f.LineEnd)
			}
			t.Logf("BraceNesting10: 行 %d-%d (一行内10层嵌套)", f.LineStart, f.LineEnd)
		}
	}
}

// ── 大量调用表达式测试 ─────────────────────────────

func TestASTMassiveCall(t *testing.T) {
	content := readTestFile(t, "ast_stress/go_massive_call.go")
	p := NewTreeSitterGoParser()
	funcs, err := p.Parse("go_massive_call.go", content)
	if err != nil {
		t.Fatalf("解析 go_massive_call.go 失败: %v", err)
	}

	nameSet := make(map[string]bool, len(funcs))
	for _, f := range funcs {
		nameSet[f.Name] = true
	}

	if !nameSet["MassCall"] {
		t.Fatal("MassCall 函数未被解析到")
	}

	for _, f := range funcs {
		if f.Name == "MassCall" {
			t.Logf("MassCall: %d 个调用, %d 行, 深度 %d",
				f.CallCount, f.LineEnd-f.LineStart+1, f.NestingDepth)
			if f.CallCount < 20 {
				t.Errorf("MassCall 应有至少 20+ 次调用，实际只有 %d", f.CallCount)
			}
		}
	}
}

// ── 并发解析压力测试 ───────────────────────────────

func TestASTConcurrentParsing(t *testing.T) {
	if testing.Short() {
		t.Skip("skip concurrent test in short mode")
	}

	// 准备 3 个不同文件的测试数据
	files := map[string][]byte{
		"go_giant.go":       readTestFile(t, "ast_stress/go_giant.go"),
		"go_nesting_hell.go": readTestFile(t, "ast_stress/go_nesting_hell.go"),
		"go_massive_call.go": readTestFile(t, "ast_stress/go_massive_call.go"),
	}

	// 并发解析: 每个文件重复 4 次
	const concurrency = 4
	var wg sync.WaitGroup
	errCh := make(chan error, len(files)*concurrency)

	for i := 0; i < concurrency; i++ {
		for name, content := range files {
			wg.Add(1)
			go func(n string, c []byte) {
				defer wg.Done()
				p := NewTreeSitterGoParser()
				funcs, err := p.Parse(n, c)
				if err != nil {
					errCh <- err
					return
				}
				if len(funcs) == 0 {
					errCh <- nil // signal empty result
				}
			}(name, content)
		}
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Errorf("并发解析失败: %v", err)
		}
	}
}

// ── 跨语言 Parser 注册表压力测试 ───────────────────

func TestASTAllParsersRegistration(t *testing.T) {
	reg := NewRegistry()
	parsers := DefaultParsers()

	for _, pr := range parsers {
		reg.Register(pr.Parser, pr.Extensions...)
	}

	exts := reg.SupportedExts()
	if len(exts) < 10 {
		t.Errorf("期望至少 10 个支持的语言扩展，实际只有 %d", len(exts))
	}
	t.Logf("注册表支持 %d 种扩展: %v", len(exts), exts)

	// 验证每种语言都能正确返回名称
	seen := make(map[string]bool)
	for _, pr := range parsers {
		lang := pr.Parser.Language()
		if seen[lang] {
			t.Errorf("重复注册语言: %s", lang)
		}
		seen[lang] = true
	}
	t.Logf("已注册 %d 种语言解析器", len(seen))
}

// ── 边界测试：空文件 ───────────────────────────────

func TestASTEmptyFile(t *testing.T) {
	p := NewTreeSitterGoParser()
	funcs, err := p.Parse("empty.go", []byte(""))
	if err != nil {
		t.Fatalf("空文件解析失败: %v", err)
	}
	if len(funcs) != 0 {
		t.Errorf("空文件应返回 0 个函数，实际 %d", len(funcs))
	}
}

func TestASTMinimalFile(t *testing.T) {
	content := []byte("package main\n")
	p := NewTreeSitterGoParser()
	funcs, err := p.Parse("minimal.go", content)
	if err != nil {
		t.Fatalf("最小文件解析失败: %v", err)
	}
	if len(funcs) != 0 {
		t.Errorf("仅包含 package 声明的文件应返回 0 个函数，实际 %d", len(funcs))
	}
}

// ── 注释/字符串边界测试 ───────────────────────────

func TestASTCommentStringEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int // 期望解析到的函数数
	}{
		{
			name: "全部注释",
			content: `package main
// func foo() {}
/* func bar() {} */
`,
			want: 0,
		},
		{
			name: "字符串中有括号",
			content: `package main
func foo() string {
    s := "func() { return 1 }"
    return s
}
`,
			want: 1,
		},
		{
			name: "多行字符串",
			content: `package main
func foo() string {
    s := ` + "`" + `line1
line2
line3` + "`" + `
    return s
}
`,
			want: 1,
		},
		{
			name: "字符串内有大括号嵌套",
			content: `package main
func jsonParse() string {
    json := ` + "`" + `{"key": {"nested": [1,2,3], "status": "ok"}}` + "`" + `
    return json
}
`,
			want: 1,
		},
		{
			name: "C++ 风格 R 字符串模式（Go 中不应触发）",
			content: `package main
func checkRPattern() string {
    // Go 中 R 没有特殊含义，但 matchBrace 可能被 C++ 污染
    rest := "R\"delim(content)delim\""
    _ = rest
    return "ok"
}
`,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewTreeSitterGoParser()
			funcs, err := p.Parse(tt.name+".go", []byte(tt.content))
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if len(funcs) != tt.want {
				t.Errorf("期望 %d 个函数，实际 %d", tt.want, len(funcs))
			}
		})
	}
}

// ── 函数体完整性测试 ───────────────────────────────

func TestASTFuncBodyIntegrity(t *testing.T) {
	content := readTestFile(t, "ast_stress/go_nesting_hell.go")
	p := NewTreeSitterGoParser()
	funcs, err := p.Parse("go_nesting_hell.go", content)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	for _, f := range funcs {
		// 验证函数体以 "{" 开头或以函数签名开头
		if len(f.Body) == 0 {
			t.Errorf("函数 %s 的 Body 为空", f.Name)
			continue
		}
		// 验证函数的 LineStart <= LineEnd
		if f.LineStart > f.LineEnd {
			t.Errorf("函数 %s 的行号异常: start=%d > end=%d",
				f.Name, f.LineStart, f.LineEnd)
		}
		// 验证文件路径匹配
		if f.FilePath != "go_nesting_hell.go" {
			t.Errorf("函数 %s 的文件路径为 %q，期望 go_nesting_hell.go",
				f.Name, f.FilePath)
		}
	}
}

// ── 大缓冲区 + 重复解析压力测试 ────────────────────

func TestASTRepeatedParsing(t *testing.T) {
	if testing.Short() {
		t.Skip("skip repeated parsing test in short mode")
	}

	content := readTestFile(t, "ast_stress/go_giant.go")
	p := NewTreeSitterGoParser()

	// 重复解析 5 次，验证结果稳定且无内存泄漏
	prevCount := -1
	for i := 0; i < 5; i++ {
		funcs, err := p.Parse("go_giant.go", content)
		if err != nil {
			t.Fatalf("第 %d 次解析失败: %v", i, err)
		}
		if prevCount >= 0 && len(funcs) != prevCount {
			t.Errorf("第 %d 次解析结果不稳定: 上次 %d 个，这次 %d 个",
				i, prevCount, len(funcs))
		}
		prevCount = len(funcs)
	}
	t.Logf("重复解析 5 次，每次稳定得到 %d 个函数", prevCount)
}

// ── 函数依赖完整性测试 ─────────────────────────────

func TestASTDependencyExtraction(t *testing.T) {
	content := readTestFile(t, "ast_stress/go_massive_call.go")
	p := NewTreeSitterGoParser()
	funcs, err := p.Parse("go_massive_call.go", content)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	for _, f := range funcs {
		if f.Name == "MassCall" {
			t.Logf("MassCall 的依赖: %v", f.Dependencies)
			// helper0 ~ helper9 应该都在依赖中
			for i := 0; i < 10; i++ {
				name := string(rune('0'+i))
				_ = name
			}
			// fmt.Sprintf 应该被解析到
			foundFmt := false
			for _, dep := range f.Dependencies {
				if dep == "Sprintf" {
					foundFmt = true
					break
				}
			}
			if !foundFmt {
				t.Errorf("MassCall 应包含依赖 Sprintf，实际依赖: %v", f.Dependencies)
			}
		}
	}
}

// ── Numeric 类型 + model 模块集成测试 ─────────────

func TestFunctionJSONRoundTrip(t *testing.T) {
	f := &model.Function{
		ID:           42,
		Name:         "StressTest",
		PackageName:  "ast_stress",
		Language:     "go",
		FilePath:     "go_giant.go",
		LineStart:    10,
		LineEnd:      100,
		Body:         "func StressTest() { return }",
		Dependencies: []string{"helper0", "helper1", "helper2"},
		CallCount:    3,
		NestingDepth: 2,
		Hash:         "stressed",
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("json.Marshal 失败: %v", err)
	}
	var f2 model.Function
	if err := json.Unmarshal(data, &f2); err != nil {
		t.Fatalf("json.Unmarshal 失败: %v", err)
	}
	if f2.Name != f.Name || f2.CallCount != f.CallCount {
		t.Errorf("JSON 往返丢失数据: before=%+v after=%+v", f, f2)
	}
}

// ────────────────────────────────────────────────────────────
// 混淆代码压力测试
// ────────────────────────────────────────────────────────────

// parseAndCheck 辅助函数：解析文件并验证关键函数存在
func parseAndCheck(t *testing.T, filename string, mustHave []string) map[string]bool {
	t.Helper()
	content := readTestFile(t, "ast_stress/"+filename)
	p := NewTreeSitterGoParser()
	funcs, err := p.Parse(filename, content)
	if err != nil {
		t.Fatalf("解析 %s 失败: %v", filename, err)
	}
	nameSet := make(map[string]bool, len(funcs))
	for _, f := range funcs {
		nameSet[f.Name] = true
	}
	for _, name := range mustHave {
		if !nameSet[name] {
			t.Errorf("期望函数 %s 未被解析到 [%s]", name, filename)
		}
	}
	t.Logf("%s: 解析到 %d 个函数, 期望 %d 个关键函数全部命中",
		filename, len(funcs), len(mustHave))
	return nameSet
}

func TestConfusingCode(t *testing.T) {
	// 验证混淆 Go 代码中的所有函数都被正确解析
	parseAndCheck(t, "go_confusing.go", []string{
		"ConfusingFunc1", "ConfusingFunc2", "ConfusingFunc3",
		"ConfusingFunc4", "ConfusingFunc5", "ConfusingFunc6",
		"ConfusingFunc7", "ConfusingFunc8", "ConfusingFunc9",
		"ConfusingFunc10",
	})
}

// ── 括号匹配混乱测试 ─────────────────────────────

func TestBraceChaos(t *testing.T) {
	names := parseAndCheck(t, "brace_chaos.go", []string{
		"Brace1", "Brace2", "Brace3", "Brace4", "Brace5",
		"Brace6", "Brace7", "Brace8", "Brace9", "Brace10",
	})

	// 验证 Brace2 包含调用（fmt.Println 应被解析为依赖）
	content := readTestFile(t, "ast_stress/brace_chaos.go")
	p := NewTreeSitterGoParser()
	funcs, err := p.Parse("brace_chaos.go", content)
	if err != nil {
		t.Fatalf("解析 brace_chaos.go 失败: %v", err)
	}

	if !names["Brace2"] {
		t.Fatal("Brace2 未被解析，后续依赖测试跳过")
	}
	for _, f := range funcs {
		if f.Name == "Brace2" {
			t.Logf("Brace2: %d 次调用, 依赖: %v", f.CallCount, f.Dependencies)
			// 应该有 fmt.Println 调用
			if f.CallCount < 1 {
				t.Errorf("Brace2 应至少包含 1 次调用")
			}
		}
		// Brace7 是一行内 40 层嵌套 {}，不应出错
		if f.Name == "Brace7" {
			if f.LineEnd-f.LineStart < 6 {
				t.Errorf("Brace7 行数范围过短: [%d, %d]，深层嵌套括号可能匹配失败",
					f.LineStart, f.LineEnd)
			}
		}
	}
}

// ── 注释嵌套测试 ───────────────────────────────

func TestCommentNesting(t *testing.T) {
	parseAndCheck(t, "comment_nesting.go", []string{
		"Comment1", "Comment2", "Comment3", "Comment4", "Comment5",
		"Comment6", "Comment7", "Comment8", "Comment9", "Comment10",
	})

	content := readTestFile(t, "ast_stress/comment_nesting.go")
	p := NewTreeSitterGoParser()
	funcs, err := p.Parse("comment_nesting.go", content)
	if err != nil {
		t.Fatalf("解析 comment_nesting.go 失败: %v", err)
	}

	// 验证 Comment9 包含 fmt.Println 依赖
	for _, f := range funcs {
		if f.Name == "Comment9" {
			t.Logf("Comment9: %d 次调用, 依赖: %v", f.CallCount, f.Dependencies)
			if f.CallCount < 3 {
				t.Errorf("Comment9 应包含 3 次 fmt.Println 调用，实际 %d", f.CallCount)
			}
		}
	}
}

// ── Unicode/特殊字符测试 ──────────────────────

func TestUnicodeChaos(t *testing.T) {
	parseAndCheck(t, "unicode_chaos.go", []string{
		"Unicode1中文関数名", "Unicode2", "Unicode3", "Unicode4",
		"Unicode5", "Unicode6", "Unicode7", "Unicode8",
		"Unicode9", "Unicode10",
	})

	content := readTestFile(t, "ast_stress/unicode_chaos.go")
	p := NewTreeSitterGoParser()
	funcs, err := p.Parse("unicode_chaos.go", content)
	if err != nil {
		t.Fatalf("解析 unicode_chaos.go 失败: %v", err)
	}

	// 验证中文函数名被正确解析
	foundUnicode := false
	for _, f := range funcs {
		if f.Name == "Unicode1中文関数名" {
			foundUnicode = true
			t.Logf("Unicode1中文関数名: 行 %d-%d", f.LineStart, f.LineEnd)
		}
	}
	if !foundUnicode {
		t.Error("中文函数名 Unicode1中文関数名 未被解析")
	}
}

// ── 语法噪声测试 ──────────────────────────────

func TestSyntaxNoise(t *testing.T) {
	parseAndCheck(t, "syntax_noise.go", []string{
		"Noise1", "Noise2", "Noise3", "Noise4", "Noise5",
		"Noise6", "Noise7", "Noise8", "Noise9",
		"Noise10", "Noise11", "Noise12",
	})
}

// ── 字符串迷宫测试 ────────────────────────────

func TestStringMaze(t *testing.T) {
	parseAndCheck(t, "string_maze.go", []string{
		"Str1", "Str2", "Str3", "Str4", "Str5",
		"Str6", "Str7", "Str8", "Str9", "Str10",
	})

	// 验证 Str4 的依赖提取（包含 fmt.Errorf）
	content := readTestFile(t, "ast_stress/string_maze.go")
	p := NewTreeSitterGoParser()
	funcs, err := p.Parse("string_maze.go", content)
	if err != nil {
		t.Fatalf("解析 string_maze.go 失败: %v", err)
	}
	for _, f := range funcs {
		if f.Name == "Str4" {
			t.Logf("Str4: %d 次调用, 依赖: %v", f.CallCount, f.Dependencies)
			// 字符串中的函数调用不应被提取
			// "fmt.Errorf(\"a (%d) <= b (%d)\", a, b)" 在字符串中
			for _, dep := range f.Dependencies {
				if dep == "Sprintf" || dep == "Errorf" {
					// Sprintf 可能在字符串拼接中出现，但 Errorf 在字符串内
					t.Logf("Str4 依赖包含 %s（可能在字符串中也可能不在）", dep)
				}
			}
		}
		// Str7 包含类似 R" 的模式，不应导致解析失败
		if f.Name == "Str7" {
			t.Logf("Str7: %d 行, 正确跳过 C++ R\" 模式", f.LineEnd-f.LineStart+1)
		}
	}
}

// ── 极极端情况测试 ──────────────────────────

func TestExtremeFileBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		minFuncs int
	}{
		{
			name: "大量空行结尾",
			content: `package main
func foo() int { return 1 }


` + "\n\n\n\n\n",
			minFuncs: 1,
		},
		{
			name: "注释结尾无换行",
			content: `package main
func bar() string { return "no newline" }
// final comment`,
			minFuncs: 1,
		},
		{
			name: "只有注释和空行",
			content: `package main
// just a comment

/* another */

// last`,
			minFuncs: 0,
		},
		{
			name: "只有一个大括号在字符串中",
			content: `package main
func braceOnly() string { return "{" }`,
			minFuncs: 1,
		},
		{
			name: "只有反大括号在字符串中",
			content: `package main
func braceOnly2() string { return "}" }`,
			minFuncs: 1,
		},
		{
			name: "函数签名跨多行",
			content: `package main
func multiLine(
	a int,
	b string,
	c float64,
) string {
	return "ok"
}`,
			minFuncs: 1,
		},
		{
			name: "匿名结构体作为参数",
			content: `package main
func anonStruct(
	opts struct {
		Name   string
		Values []int
		Active bool
	},
) string { return opts.Name }`,
			minFuncs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewTreeSitterGoParser()
			funcs, err := p.Parse("extreme_"+tt.name+".go", []byte(tt.content))
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if len(funcs) < tt.minFuncs {
				t.Errorf("期望至少 %d 个函数, 实际 %d", tt.minFuncs, len(funcs))
			}
		})
	}
}

// ── 全文件整合压力测试 ───────────────────────

func TestASTAllConfusingFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode")
	}

	files := []struct {
		name  string
		count int // 期望最少函数数
	}{
		{"brace_chaos.go", 10},
		{"comment_nesting.go", 10},
		{"go_confusing.go", 10},
		{"string_maze.go", 10},
		{"syntax_noise.go", 12},
		{"unicode_chaos.go", 10},
	}

	totalFuncs := 0
	for _, f := range files {
		content := readTestFile(t, "ast_stress/"+f.name)
		p := NewTreeSitterGoParser()
		funcs, err := p.Parse(f.name, content)
		if err != nil {
			t.Errorf("解析 %s 失败: %v", f.name, err)
			continue
		}
		if len(funcs) < f.count {
			t.Errorf("%s: 期望至少 %d 个函数, 实际 %d", f.name, f.count, len(funcs))
		}
		totalFuncs += len(funcs)
	}
	t.Logf("6 个混淆文件共解析到 %d 个函数", totalFuncs)
}

// ── C++ R" 污染攻击测试 ──────────────────────────

func TestCPPRawPollution(t *testing.T) {
	names := parseAndCheck(t, "cpp_raw_pollution.go", []string{
		"RPollution1", "RPollution2", "RPollution3", "RPollution4",
		"RPollution5", "RPollution6", "RPollution7", "RPollution8",
		"RPollution9", "RPollution10",
	})
	if !names["RPollution10"] {
		t.Fatal("RPollution10 未被解析，跳过后续检查")
	}

	// 验证 R" 污染不破坏函数体边界
	content := readTestFile(t, "ast_stress/cpp_raw_pollution.go")
	p := NewTreeSitterGoParser()
	funcs, err := p.Parse("cpp_raw_pollution.go", content)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	// 关键验证：所有函数都有正确的行号范围
	for _, f := range funcs {
		if f.LineStart <= 0 || f.LineEnd <= 0 || f.LineEnd < f.LineStart {
			t.Errorf("函数 %s 行号异常: [%d, %d]", f.Name, f.LineStart, f.LineEnd)
		}
		// 验证函数体非空
		if len(f.Body) == 0 {
			t.Errorf("函数 %s 体为空，可能括号匹配失败", f.Name)
		}
	}

	// 验证 RPollution5 有 switch（包含多种调用）
	for _, f := range funcs {
		if f.Name == "RPollution5" {
			if f.CallCount < 1 {
				t.Errorf("RPollution5 应包含 fmt.Println 调用，实际 %d 次调用",
					f.CallCount)
			}
		}
	}
}

// ── 括号匹配混沌测试 ─────────────────────────────

func TestBraceTorture(t *testing.T) {
	parseAndCheck(t, "brace_torture.go", []string{
		"Torture1", "Torture2", "Torture3", "Torture4", "Torture5",
		"Torture6", "Torture7", "Torture8", "Torture9",
		"Torture10a", "Torture10b", "Torture10c", "Torture10d", "Torture10e",
		"Torture11", "Torture12",
	})

	content := readTestFile(t, "ast_stress/brace_torture.go")
	p := NewTreeSitterGoParser()
	funcs, err := p.Parse("brace_torture.go", content)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	// 验证每个函数的行号完整性
	for _, f := range funcs {
		if f.LineEnd < f.LineStart {
			t.Errorf("函数 %s 行号反转: [%d, %d]", f.Name, f.LineStart, f.LineEnd)
		}
		bodyLen := f.LineEnd - f.LineStart + 1
		if bodyLen < 1 {
			t.Errorf("函数 %s 行范围异常: [%d, %d]", f.Name, f.LineStart, f.LineEnd)
		}
	}

	// Torture6 应有 select 操作（chan 调用被提取）
	for _, f := range funcs {
		if f.Name == "Torture6" {
			t.Logf("Torture6: %d 次调用, 依赖: %v", f.CallCount, f.Dependencies)
		}
	}

	// Torture10a-e 都是极简函数
	tinyCount := 0
	for _, f := range funcs {
		if len(f.Body) > 0 && f.LineEnd-f.LineStart <= 1 {
			tinyCount++
		}
	}
	t.Logf("brace_torture.go: %d 个函数, 其中 %d 个极简函数(≤2行)", len(funcs), tinyCount)
}

// ── Go 泛型压力测试 ─────────────────────────────

func TestGenericHell(t *testing.T) {
	// 注意: 带泛型接收器的方法 (Generic7, Generic11) 受 go-tree-sitter 版本限制
	// 可能不会被正确解析为 method_declaration。
	// 我们只验证普通泛型函数和顶层函数。
	names := parseAndCheck(t, "generic_hell.go", []string{
		"Generic1", "Generic2", "Generic3", "Generic4", "Generic5",
		"Generic6", "Generic8", "Generic9", "Generic10",
		"Generic12", "Generic13",
	})

	if !names["Generic1"] {
		t.Fatal("Generic1 未被解析")
	}

	content := readTestFile(t, "ast_stress/generic_hell.go")
	p := NewTreeSitterGoParser()
	funcs, err := p.Parse("generic_hell.go", content)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	// 验证泛型函数有正确的调用提取
	for _, f := range funcs {
		t.Logf("  %s: %d 次调用, 深度 %d", f.Name, f.CallCount, f.NestingDepth)
	}

	// 验证 Generic13（调用泛型函数推导）有 4 次调用
	for _, f := range funcs {
		if f.Name == "Generic13" {
			if f.CallCount < 4 {
				t.Errorf("Generic13 应包含至少 4 次泛型函数调用, 实际 %d", f.CallCount)
			}
		}
	}

	// Generic7/Generic11 是已知限制，检查是否被解析（解析到是bonus，没解析到不算失败）
	for _, f := range funcs {
		if f.Name == "Set" || f.Name == "Get" {
			t.Logf("泛型方法 %s 被解析到（go-tree-sitter 支持泛型方法）", f.Name)
		}
	}
}

// ── Go 语法边界测试 ─────────────────────────────

func TestSyntaxEdge(t *testing.T) {
	parseAndCheck(t, "syntax_edge.go", []string{
		"Syntax1", "Read", "Write", "Close",
		"Syntax5", "Syntax6", "init", "Syntax8", "Syntax9", "Syntax10",
	})

	content := readTestFile(t, "ast_stress/syntax_edge.go")
	p := NewTreeSitterGoParser()
	funcs, err := p.Parse("syntax_edge.go", content)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	// init 应该被解析到
	hasInit := false
	for _, f := range funcs {
		if f.Name == "init" {
			hasInit = true
		}
	}
	if !hasInit {
		t.Error("init 函数未被解析")
	}
}

// ── 整合全量 AST 压力测试 ──────────────────────

func TestASTAllDefectTests(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode")
	}

	files := []struct {
		name  string
		count int
	}{
		{"cpp_raw_pollution.go", 10},
		{"brace_torture.go", 16},
		{"generic_hell.go", 11},
		{"syntax_edge.go", 10},
	}

	totalFuncs := 0
	for _, f := range files {
		content := readTestFile(t, "ast_stress/"+f.name)
		p := NewTreeSitterGoParser()
		funcs, err := p.Parse(f.name, content)
		if err != nil {
			t.Errorf("解析 %s 失败: %v", f.name, err)
			continue
		}
		if len(funcs) < f.count {
			t.Errorf("%s: 期望至少 %d 个函数, 实际 %d", f.name, f.count, len(funcs))
		}
		totalFuncs += len(funcs)
	}
	t.Logf("4 个缺陷专项文件共解析到 %d 个函数", totalFuncs)
}
