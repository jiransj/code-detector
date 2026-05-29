package parser

import (
	"strings"
	"testing"
)

// ──────────────────────────────
// FileLines 测试
// ──────────────────────────────

func TestNewFileLines(t *testing.T) {
	text := "line0\nline1\nline2\n"
	fl := NewFileLines(text)

	// 4 offsets: [0, 5, 10, 15] → 最后一个是空行, Split 产生 4 个元素
	if fl.NumLines() != 4 {
		t.Fatalf("expected 4 lines (incl. trailing empty), got %d", fl.NumLines())
	}

	// LineOffset
	if off := fl.LineOffset(0); off != 0 {
		t.Errorf("LineOffset(0) = %d, want 0", off)
	}
	if off := fl.LineOffset(1); off != 6 {
		t.Errorf("LineOffset(1) = %d, want 6", off)
	}

	// LineFromOffset
	if idx := fl.LineFromOffset(0); idx != 0 {
		t.Errorf("LineFromOffset(0) = %d, want 0", idx)
	}
	if idx := fl.LineFromOffset(6); idx != 1 {
		t.Errorf("LineFromOffset(6) = %d, want 1", idx)
	}

	// Lines
	lines := fl.Lines()
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}
	if lines[0] != "line0" {
		t.Errorf("lines[0] = %q, want 'line0'", lines[0])
	}
	if lines[3] != "" {
		t.Errorf("lines[3] = %q, want '' (trailing)", lines[3])
	}

	// LineContent
	if content := fl.LineContent(1); content != "line1" {
		t.Errorf("LineContent(1) = %q, want 'line1'", content)
	}
}

func TestNewFileLinesNoTrailingNewline(t *testing.T) {
	text := "line0\nline1"
	fl := NewFileLines(text)
	if fl.NumLines() != 2 {
		t.Fatalf("expected 2 lines, got %d", fl.NumLines())
	}
}

func TestNewFileLinesEmpty(t *testing.T) {
	fl := NewFileLines("")
	if fl.NumLines() != 1 {
		t.Fatalf("expected 1 line for empty string, got %d", fl.NumLines())
	}
	lines := fl.Lines()
	if len(lines) != 1 || lines[0] != "" {
		t.Errorf("expected one empty string, got %v", lines)
	}
}

func TestLineContent(t *testing.T) {
	text := "abc\ndef\n"
	fl := NewFileLines(text)
	if c := fl.LineContent(0); c != "abc" {
		t.Errorf("LineContent(0)=%q, want 'abc'", c)
	}
	if c := fl.LineContent(1); c != "def" {
		t.Errorf("LineContent(1)=%q, want 'def'", c)
	}
}

// ──────────────────────────────
// matchBrace 测试
// ──────────────────────────────

func TestMatchBraceSimple(t *testing.T) {
	text := "func foo() { return 1; }"
	openPos := strings.Index(text, "{")
	if openPos < 0 {
		t.Fatal("no brace found")
	}
	closePos, err := matchBrace(text, openPos)
	if err != nil {
		t.Fatalf("matchBrace error: %v", err)
	}
	if text[closePos] != '}' {
		t.Errorf("expected '}', got %c at %d", text[closePos], closePos)
	}
}

func TestMatchBraceNested(t *testing.T) {
	text := "func() { if (true) { for {} } }"
	openPos := strings.Index(text, "{")
	if openPos < 0 {
		t.Fatal("no brace found")
	}
	closePos, err := matchBrace(text, openPos)
	if err != nil {
		t.Fatalf("matchBrace error: %v", err)
	}
	if text[closePos] != '}' {
		t.Errorf("expected '}', got %c", text[closePos])
	}
	lastBrace := strings.LastIndex(text, "}")
	if closePos != lastBrace {
		t.Errorf("expected close at %d (last brace), got %d", lastBrace, closePos)
	}
}

func TestMatchBraceStrings(t *testing.T) {
	text := "f() { s := \"{\"; return 1; }"
	openPos := strings.Index(text, "{")
	closePos, err := matchBrace(text, openPos)
	if err != nil {
		t.Fatalf("matchBrace error: %v", err)
	}
	lastBrace := strings.LastIndex(text, "}")
	if closePos != lastBrace {
		t.Errorf("expected close at %d, got %d", lastBrace, closePos)
	}
}

func TestMatchBraceUnmatched(t *testing.T) {
	text := "func() { return; "
	openPos := strings.Index(text, "{")
	_, err := matchBrace(text, openPos)
	if err == nil {
		t.Errorf("expected error for unmatched brace, got nil")
	}
}

func TestMatchBraceNotABrace(t *testing.T) {
	_, err := matchBrace("hello world", 0)
	if err == nil {
		t.Errorf("expected error for non-brace position, got nil")
	}
}

// ──────────────────────────────
// makeCommentMask 测试
// ──────────────────────────────

func TestMakeCommentMaskLineComments(t *testing.T) {
	lines := []string{
		"var x = 1",
		"// this is a comment",
		"var y = 2",
	}
	mask := makeCommentMask(lines, []string{"//"}, [][2]string{})
	if mask[0] {
		t.Errorf("line 0 should not be masked")
	}
	if !mask[1] {
		t.Errorf("line 1 should be masked (comment)")
	}
	if mask[2] {
		t.Errorf("line 2 should not be masked")
	}
}

func TestMakeCommentMaskBlockComments(t *testing.T) {
	lines := []string{
		"var x = 1",
		"/* start block",
		"inside block",
		"end */",
		"var z = 3",
	}
	mask := makeCommentMask(lines, []string{"//"}, [][2]string{{"/*", "*/"}})
	if mask[0] {
		t.Errorf("line 0 should not be masked")
	}
	if !mask[1] {
		t.Errorf("line 1 should be masked (block comment start)")
	}
	if !mask[2] {
		t.Errorf("line 2 should be masked (inside block comment)")
	}
	if !mask[3] {
		t.Errorf("line 3 should be masked (end of block comment)")
	}
	if mask[4] {
		t.Errorf("line 4 should not be masked")
	}
}

// ──────────────────────────────
// extractCallStatsSimple 测试
// ──────────────────────────────

func TestExtractCallStatsSimple(t *testing.T) {
	// 验证正则捕获组: (?:(\w+)\.)?(\w+)\s*\(
	// group 1 = prefix, group 2 = name
	matches := genericCallRegex.FindAllStringSubmatch("doSomething(arg1, arg2)", -1)
	if len(matches) == 0 {
		t.Fatal("genericCallRegex does not match 'doSomething(arg1, arg2)'")
	}
	if len(matches[0]) < 3 {
		t.Fatalf("expected at least 3 match groups, got %d: %v", len(matches[0]), matches[0])
	}
	// group 1 = prefix (empty, no dot), group 2 = function name
	if matches[0][2] != "doSomething" {
		t.Errorf("expected name 'doSomething' in group 2, got %q (groups: %v)", matches[0][2], matches[0])
	}

	// 验证带前缀匹配: fmt.Sprintf
	matches2 := genericCallRegex.FindAllStringSubmatch("fmt.Sprintf(\"%s\", x)", -1)
	if len(matches2) > 0 && len(matches2[0]) >= 3 {
		if matches2[0][1] != "fmt" {
			t.Errorf("expected prefix 'fmt' in group 1, got %q", matches2[0][1])
		}
		if matches2[0][2] != "Sprintf" {
			t.Errorf("expected name 'Sprintf' in group 2, got %q", matches2[0][2])
		}
	}

	// 验证 extractCallStatsSimple 能正确识别函数调用
	body := "doSomething(arg1, arg2)\nhelper()\n"
	callRegex := genericCallRegex
	skipFn := func(name string) bool { return false }

	stats := extractCallStatsSimple(body, callRegex, skipFn)
	if stats.CallCount == 0 {
		t.Errorf("expected some calls, got 0")
	}

	foundDoSomething := false
	foundHelper := false
	for _, callee := range stats.Callees {
		if callee == "doSomething" {
			foundDoSomething = true
		}
		if callee == "helper" {
			foundHelper = true
		}
	}
	if !foundDoSomething {
		t.Errorf("expected doSomething in callees, got %v", stats.Callees)
	}
	if !foundHelper {
		t.Errorf("expected helper in callees, got %v", stats.Callees)
	}
}

func TestExtractCallStatsSimpleSkipFn(t *testing.T) {
	body := "Sprintf(\"format %d\", val)\nhelper()\n"
	callRegex := genericCallRegex
	skipFn := func(name string) bool { return name == "Sprintf" }

	stats := extractCallStatsSimple(body, callRegex, skipFn)
	for _, callee := range stats.Callees {
		if callee == "Sprintf" {
			t.Errorf("Sprintf should be skipped by skipFn, got callees: %v", stats.Callees)
		}
	}
	// helper should still be found
	foundHelper := false
	for _, callee := range stats.Callees {
		if callee == "helper" {
			foundHelper = true
		}
	}
	if !foundHelper {
		t.Errorf("expected helper in callees, got %v", stats.Callees)
	}
}

// ──────────────────────────────
// goVisibility 测试
// ──────────────────────────────

func TestGoVisibility(t *testing.T) {
	if v := goVisibility("Foo"); v != "public" {
		t.Errorf("goVisibility(Foo) = %s, want public", v)
	}
	if v := goVisibility("foo"); v != "private" {
		t.Errorf("goVisibility(foo) = %s, want private", v)
	}
	if v := goVisibility(""); v != "private" {
		t.Errorf("goVisibility('') = %s, want private", v)
	}
}
