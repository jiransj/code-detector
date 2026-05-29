package parser

import (
	"regexp"
	"strings"

	"code-detector/internal/model"
)

// RustParser 解析 Rust 源文件
type RustParser struct{}

func NewRustParser() *RustParser { return &RustParser{} }
func (p *RustParser) Language() string { return "rust" }

func (p *RustParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	text := string(content)
	lines := strings.Split(text, "\n")
	commentMask := makeCommentMask(lines, []string{"//"}, [][2]string{{"/*", "*/"}})

	var vars []*model.GlobalVariable
	// static NAME: type = value;
	// const NAME: type = value;
	rustGlobalRegex := regexp.MustCompile(`^\s*(?:(?:pub\s+)?(?:static|const))\s+(?P<name>\w+)\s*:\s*(?P<type>[^=;]+)\s*(?:=|;)`)

	for i, line := range lines {
		if commentMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		matches := rustGlobalRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := rustGlobalRegex.SubexpIndex("name")
		typeIdx := rustGlobalRegex.SubexpIndex("type")
		name := matches[nameIdx]
		varType := ""
		if typeIdx >= 0 && typeIdx < len(matches) {
			varType = strings.TrimSpace(matches[typeIdx])
		}
		if name != "" {
			isConst := strings.Contains(trimmed, " const ")
			vars = append(vars, &model.GlobalVariable{
				Name: name, VarType: varType, Language: "rust",
				FilePath: filePath, LineNum: i + 1, IsConst: isConst,
			})
		}
	}
	return vars, nil
}

var rustFuncRegex = regexp.MustCompile(
	`(?:pub\s+(?:unsafe\s+)?)?(?:unsafe\s+)?fn\s+(?P<name>\w+)\s*\(`,
)
var rustCallRegex = regexp.MustCompile(`(?:(\w+)::)?(\w+)\s*\(`)

func (p *RustParser) Parse(filePath string, content []byte) ([]*model.Function, error) {
	fl := NewFileLinesFromBytes(content)
	lines := fl.Lines()
	text := fl.Text()

	commentMask := makeCommentMask(lines, []string{"//"}, [][2]string{{"/*", "*/"}})
	stringMask := makeStringMask(lines)

	type rsFuncStart struct {
		lineIdx int
		name    string
	}
	var starts []rsFuncStart

	for i, line := range lines {
		if commentMask[i] || stringMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		matches := rustFuncRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := rustFuncRegex.SubexpIndex("name")
		if nameIdx < 0 || nameIdx >= len(matches) {
			continue
		}
		name := matches[nameIdx]
		if name == "" {
			continue
		}

		starts = append(starts, rsFuncStart{lineIdx: i, name: name})
	}

	if len(starts) == 0 {
		return nil, nil
	}

	allFuncs := make([]*model.Function, 0, len(starts))
	for _, fs := range starts {
		// Rust 函数体可能以 { 开始，也可能以 where 子句 + { 开始
		braceOffset := -1
		for j := fs.lineIdx; j < len(lines); j++ {
			line := lines[j]
			idx := strings.Index(line, "{")
			if idx >= 0 && !commentMask[j] {
				braceOffset = fl.LineOffset(j) + idx
				break
			}
		}
		if braceOffset < 0 {
			continue
		}

		closeOffset, err := matchBrace(text, braceOffset)
		if err != nil {
			continue
		}

		startLine := fs.lineIdx
		endLine := fl.LineFromOffset(closeOffset)

		bodyStart := fl.LineOffset(startLine)
		bodyEnd := closeOffset + 1
		if bodyEnd > len(text) {
			bodyEnd = len(text)
		}
		body := text[bodyStart:bodyEnd]

		callStats := extractCallStats(body, rustCallRegex, stringMask, commentMask, startLine, endLine, isRustStdFunction, nil)

		f := &model.Function{
			Name:         fs.name,
			Language:     "rust",
			FilePath:     filePath,
			LineStart:    startLine + 1,
			LineEnd:      endLine + 1,
			Body:         body,
			Dependencies: callStats.Callees,
			CallCount:    callStats.CallCount,
			NestingDepth: callStats.NestingDepth,
		}
		allFuncs = append(allFuncs, f)
	}

	return allFuncs, nil
}

func isRustKeyword(name string) bool {
	switch name {
	case "if", "else", "for", "while", "loop",
		"match", "return", "break", "continue",
		"let", "mut", "const", "static",
		"fn", "impl", "trait", "struct",
		"enum", "type", "union", "mod",
		"use", "pub", "crate", "self", "super",
		"where", "as", "in", "ref",
		"move", "async", "await", "unsafe",
		"dyn", "abstract", "become", "box",
		"do", "final", "macro", "override",
		"priv", "typeof", "unsized", "virtual",
		"yield", "try", "macro_rules",
		"Some", "None", "Ok", "Err",
		"String", "str", "Vec", "Box",
		"Option", "Result", "HashMap",
		"i32", "i64", "u32", "u64",
		"f32", "f64", "bool", "char",
		"usize", "isize", "u8", "i8",
		"println", "print", "format", "panic",
		"assert", "assert_eq", "assert_ne",
		"unreachable", "unimplemented", "todo",
		"dbg", "vec", "write", "writeln":
		return true
	}
	return false
}

func isRustStdFunction(name string) bool {
	switch name {
	case "if", "else", "for", "while", "loop",
		"match", "return", "break", "continue",
		"let", "mut", "const", "static",
		"fn", "impl", "trait", "struct",
		"enum", "type", "union", "mod",
		"use", "pub", "crate", "self", "super",
		"where", "as", "in", "ref",
		"move", "async", "await", "unsafe",
		"dyn", "abstract", "become", "box",
		"do", "final", "macro", "override",
		"priv", "typeof", "unsized", "virtual",
		"yield", "try", "macro_rules",
		"Some", "None", "Ok", "Err",
		"String", "str", "Vec", "Box",
		"Option", "Result", "HashMap", "HashSet",
		"BTreeMap", "BTreeSet", "LinkedList", "VecDeque",
		"Rc", "Arc", "Cell", "RefCell", "Mutex", "RwLock",
		"i32", "i64", "u32", "u64", "i128", "u128",
		"f32", "f64", "bool", "char",
		"usize", "isize", "u8", "i8", "u16", "i16",
		"println", "print", "format", "panic",
		"assert", "assert_eq", "assert_ne",
		"unreachable", "unimplemented", "todo",
		"dbg", "vec", "write", "writeln",
		// Iterator
		"map", "filter", "fold", "reduce", "for_each",
		"collect", "partition", "find", "position",
		"any", "all", "count", "sum", "product",
		"min", "max", "min_by", "max_by",
		"cloned", "copied", "chain", "zip", "enumerate",
		"skip", "take", "skip_while", "take_while",
		"flat_map", "flatten", "filter_map",
		"rev", "cycle", "peekable",
		// 常用标准库
		"clone", "copy", "as_ref", "as_mut", "into",
		"from", "default", "new", "len", "is_empty",
		"contains", "starts_with", "ends_with",
		"push", "pop", "insert", "remove",
		"append", "clear", "reserve", "resize",
		"sort", "sort_by", "sort_by_key", "reverse",
		"split", "split_at", "split_off",
		"join", "concat", "repeat", "replace",
		"trim", "trim_start", "trim_end",
		"parse", "to_string", "to_owned",
		"unwrap", "expect", "ok", "err",
		"is_some", "is_none", "is_ok", "is_err",
		"file!", "line!", "column!", "include_str!", "include_bytes!",
		"stringify!", "compile_error!", "concat!",
		"env!", "option_env!", "cfg!", "cfg_attr!":
		return true
	}
	return false
}
