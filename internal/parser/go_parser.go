package parser

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"

	"code-detector/internal/model"
)

// GoParser 解析 Go 源文件
type GoParser struct{}

func NewGoParser() *GoParser { return &GoParser{} }

func (p *GoParser) Language() string { return "go" }

// goVarRegex 匹配包级别 var 声明
var goVarRegex = regexp.MustCompile(`^\s*var\s+(?P<name>\w+)\s*(?:(?P<type>\w+(?:\[\])*(?:<[^>]*>)?))?(?:\s*=)?`)

// goConstRegex 匹配包级别 const 声明
var goConstRegex = regexp.MustCompile(`^\s*const\s+(?P<name>\w+)\s*(?:(?P<type>\w+(?:\[\])*))?(?:\s*=)?`)

// goVisibility 根据首字母判断可见性
func goVisibility(name string) string {
	if name == "" {
		return "private"
	}
	r := []rune(name)
	if unicode.IsUpper(r[0]) {
		return "public"
	}
	return "private"
}

// makeGoFuncBodyMask 标记哪些行在函数体内部（排除局部变量）
// 只处理 func 关键字后的 {...} 块，正确处理 { 在下一行的情况
func makeGoFuncBodyMask(lines []string) []bool {
	mask := make([]bool, len(lines))
	depth := 0
	inFunc := false
	braceFound := false
	bodyStarted := false // 是否已经开始追踪函数体行（从 { 的下一行开始）
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if !inFunc && (strings.HasPrefix(trimmed, "func ") || strings.HasPrefix(trimmed, "func(")) {
			inFunc = true
			braceFound = false
			bodyStarted = false
		}

		if !inFunc {
			continue
		}

		// 先检查当前行是否有 {，但 bodyStarted 在 { 之后才启用
		hadBrace := false
		for _, ch := range line {
			if ch == '{' && !braceFound {
				braceFound = true
				depth = 1
				hadBrace = true
			} else if ch == '{' && braceFound {
				depth++
			} else if ch == '}' {
				depth--
			}
		}

		// 如果 { 在此行才首次出现，此行是声明行，不标记
		if hadBrace {
			bodyStarted = true
			// 对于单行函数（如 func Foo() { return 1 }），{ 和 } 在同一行
			// depth 可能已归零，需要关闭 inFunc
			if depth <= 0 {
				inFunc = false
			}
			continue
		}

		if !bodyStarted {
			continue
		}

		mask[i] = true

		if depth <= 0 {
			inFunc = false
		}
	}
	return mask
}

// goFuncRegex 匹配 Go 函数/方法定义
var goFuncRegex = regexp.MustCompile(
	`(?:func\s+)(?:(?P<receiver>\s*\([^)]*\))\s+)?(?P<name>\w+)\s*\(`,
)

// goCallRegex 匹配函数调用（含可选接收者前缀，双捕获组支持 qualified call 免过滤）
var goCallRegex = regexp.MustCompile(`(?:(\w+)\.)?(\w+)\s*\(`)

func (p *GoParser) Parse(filePath string, content []byte) ([]*model.Function, error) {
	fl := NewFileLinesFromBytes(content)
	lines := fl.Lines()
	text := fl.Text()

	commentMask := makeCommentMask(lines, []string{"//"}, [][2]string{{"/*", "*/"}})
	stringMask := makeStringMask(lines)
	pkgName := extractGoPackageName(lines, commentMask)

	type goFuncStart struct {
		matchStart int // 匹配位置（func 关键字附近的偏移）
		name       string
		lineIdx    int
	}
	var starts []goFuncStart

	for i, line := range lines {
		if commentMask[i] || stringMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}
		if strings.HasPrefix(trimmed, "go ") {
			continue
		}

		matches := goFuncRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := goFuncRegex.SubexpIndex("name")
		if nameIdx < 0 || nameIdx >= len(matches) {
			continue
		}
		name := matches[nameIdx]
		if name == "" || name == "init" {
			continue
		}

		funcIdx := strings.Index(line, "func ")
		if funcIdx < 0 {
			funcIdx = strings.Index(line, "func(")
		}
		if funcIdx < 0 {
			funcIdx = 0
		}
		matchStart := fl.LineOffset(i) + funcIdx
		starts = append(starts, goFuncStart{matchStart: matchStart, name: name, lineIdx: i})
	}

	if len(starts) == 0 {
		return nil, nil
	}

	allFuncs := make([]*model.Function, 0, len(starts))

	for _, fs := range starts {
		// 从函数匹配位置之后找 "{", 而不是整行的第一个 "{"
		// 避免同一行上 struct/interface 等前置构造体的括号被误匹配
		braceLine := fs.lineIdx
		funcIdxInLine := fs.matchStart - fl.LineOffset(braceLine)
		afterFunc := lines[braceLine][funcIdxInLine:]
		braceIdxInLine := strings.Index(afterFunc, "{")
		if braceIdxInLine < 0 {
			// 当前行函数名后没找到 "{", 可能在后续行
			for j := braceLine + 1; j < len(lines); j++ {
				if commentMask[j] || stringMask[j] {
					continue
				}
				if idx := strings.Index(lines[j], "{"); idx >= 0 {
					braceLine = j
					braceIdxInLine = idx
					break
				}
			}
			if braceIdxInLine < 0 {
				continue
			}
		} else {
			braceIdxInLine += funcIdxInLine
		}
		offset := fl.LineOffset(braceLine) + braceIdxInLine

		closeOffset, err := matchBrace(text, offset)
		if err != nil {
			continue
		}

		startLine := fl.LineFromOffset(fs.matchStart)
		endLine := fl.LineFromOffset(closeOffset)

		bodyStart := fs.matchStart
		bodyEnd := closeOffset + 1
		if bodyEnd > len(text) {
			bodyEnd = len(text)
		}
		if bodyEnd < bodyStart {
			// matchBrace 返回的 closeOffset 在 matchStart 之前，说明括号匹配失败
			// 跳过此函数以避免空 body 入库或 panic
			if DebugMode {
				fmt.Fprintf(os.Stderr, "debug: go_parser: brace mismatch at %s:%d~%d (matchStart=%d, closeOffset=%d), skipping func %s\n",
					filePath, startLine+1, endLine+1, fs.matchStart, closeOffset, fs.name)
			}
			continue
		}
		body := text[bodyStart:bodyEnd]

		callStats := extractCallStats(body, goCallRegex, stringMask, commentMask, startLine, endLine, isKeyword, isAllUpper)

		f := &model.Function{
			Name:         fs.name,
			PackageName:  pkgName,
			Language:     "go",
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

// Globals 提取 Go 文件中的全局变量和常量
func (p *GoParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	text := string(content)
	lines := strings.Split(text, "\n")
	commentMask := makeCommentMask(lines, []string{"//"}, [][2]string{{"/*", "*/"}})
	stringMask := makeStringMask(lines)
	funcBodyMask := makeGoFuncBodyMask(lines)

	var vars []*model.GlobalVariable

	varDeclRegex := regexp.MustCompile(`^\s*(?:var|const)\s+(?P<name>\w+)\s*(?:(?P<type>\w+(?:\[\])*(?:<[^>]*>)?))?\s*(?:=|;|=)`)

	for i, line := range lines {
		if commentMask[i] || stringMask[i] || funcBodyMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		matches := varDeclRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := varDeclRegex.SubexpIndex("name")
		typeIdx := varDeclRegex.SubexpIndex("type")
		if nameIdx < 0 || nameIdx >= len(matches) {
			continue
		}
		name := matches[nameIdx]
		if name == "" || name == "(" {
			continue
		}
		varType := ""
		if typeIdx >= 0 && typeIdx < len(matches) {
			varType = matches[typeIdx]
		}
		vars = append(vars, &model.GlobalVariable{
			Name:        name,
			VarType:     varType,
			Language:    "go",
			PackageName: extractGoPackageName(lines, commentMask),
			Visibility:  goVisibility(name),
			FilePath:    filePath,
			LineNum:     i + 1,
			IsConst:     strings.HasPrefix(trimmed, "const"),
		})
	}
	return vars, nil
}

func extractGoPackageName(lines []string, commentMask []bool) string {
	for i, line := range lines {
		if commentMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}

func extractGlobalsFromLines(lines []string, commentMask []bool, varRegex *regexp.Regexp, language string, filePath string, funcBodyMask []bool, stringMask []bool) []*model.GlobalVariable {
	var vars []*model.GlobalVariable
	for i, line := range lines {
		if commentMask[i] || stringMask[i] || funcBodyMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}
		matches := varRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := varRegex.SubexpIndex("name")
		if nameIdx < 0 || nameIdx >= len(matches) {
			continue
		}
		name := matches[nameIdx]
		if name == "" || name == "(" {
			continue
		}
		typeIdx := varRegex.SubexpIndex("type")
		varType := ""
		if typeIdx >= 0 && typeIdx < len(matches) {
			varType = matches[typeIdx]
		}
		vars = append(vars, &model.GlobalVariable{
			Name:     name,
			VarType:  varType,
			Language: language,
			FilePath: filePath,
			LineNum:  i + 1,
		})
	}
	return vars
}

// isAllUpper 判断是否全大写（通常为常量）
func isAllUpper(s string) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= 'a' && b <= 'z' {
			return false
		}
		if b < 0x80 {
			continue
		}
		for _, r := range s[i:] {
			if unicode.IsLetter(r) && !unicode.IsUpper(r) {
				return false
			}
		}
		return true
	}
	return true
}

// makeCommentMask 标记哪些行处于注释中（块注释跨行）
func makeCommentMask(lines []string, singleComments []string, blockComments [][2]string) []bool {
	mask := make([]bool, len(lines))
	inBlock := false

	for i, line := range lines {
		if inBlock {
			mask[i] = true
			for _, bc := range blockComments {
				if idx := strings.Index(line, bc[1]); idx >= 0 {
					inBlock = false
					// 检查结束标记后是否还有单行注释开始
					afterEnd := line[idx+len(bc[1]):]
					for _, sc := range singleComments {
						if strings.Contains(afterEnd, sc) {
							mask[i] = true
							break
						}
					}
					break
				}
			}
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// 检查行首单行注释
		for _, sc := range singleComments {
			if strings.HasPrefix(trimmed, sc) {
				mask[i] = true
				goto nextLine
			}
		}

		// 检查块注释开始
		for _, bc := range blockComments {
			if startIdx := strings.Index(line, bc[0]); startIdx >= 0 {
				beforeStart := line[:startIdx]
				isInString := false
				for _, ch := range beforeStart {
					if ch == '"' || ch == '\'' || ch == '`' {
						isInString = !isInString
					}
				}
				if isInString {
					continue
				}
				if endIdx := strings.Index(line[startIdx+len(bc[0]):], bc[1]); endIdx >= 0 {
					mask[i] = true
				} else {
					mask[i] = true
					inBlock = true
				}
				break
			}
		}

	nextLine:
	}
	return mask
}

// makeStringMask 标记跨行字符串字面量
func makeStringMask(lines []string) []bool {
	mask := make([]bool, len(lines))
	for i, line := range lines {
		inDouble := false
		inSingle := false
		inBacktick := false
		escaped := false

		for _, ch := range line {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' && (inDouble || inSingle) {
				escaped = true
				continue
			}
			if ch == '"' && !inSingle && !inBacktick {
				inDouble = !inDouble
			} else if ch == '\'' && !inDouble && !inBacktick {
				inSingle = !inSingle
			} else if ch == '`' && !inDouble && !inSingle {
				inBacktick = !inBacktick
			}
		}
		mask[i] = inDouble || inSingle || inBacktick
	}
	return mask
}

func hasSingleCommentPrefix(s string, singleComments []string, offset int) bool {
	for _, sc := range singleComments {
		if offset+len(sc) <= len(s) && s[offset:offset+len(sc)] == sc {
			return true
		}
	}
	return false
}

// FileMasks 聚合的行掩码
type FileMasks struct {
	CommentMask []bool
	StringMask  []bool
}

func buildMasks(lines []string, singleComments []string, blockComments [][2]string) FileMasks {
	return FileMasks{
		CommentMask: makeCommentMask(lines, singleComments, blockComments),
		StringMask:  makeStringMask(lines),
	}
}
// goKeywords 是 Go 语言关键字和常用标准库名的集合（map 数据驱动）
var goKeywords = map[string]bool{	"if": true, "for": true, "switch": true, "select": true, "case": true, "default": true,
	"return": true, "go": true, "defer": true, "range": true, "break": true, "continue": true,
	"fallthrough": true, "else": true, "map": true, "chan": true, "type": true, "struct": true,
	"interface": true, "func": true, "make": true, "new": true, "append": true, "len": true,
	"cap": true, "copy": true, "close": true, "delete": true, "panic": true, "recover": true,
	"print": true, "println": true, "error": true, "nil": true, "true": true, "false": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true, "complex64": true, "complex128": true,
	"byte": true, "rune": true, "string": true, "bool": true, "uintptr": true,
	"Printf": true, "Fprint": true, "Fprintf": true, "Fprintln": true,
	"Sprint": true, "Sprintf": true, "Sprintln": true, "Errorf": true,
	"Scanf": true, "Scan": true, "Scanln": true, "Fscanf": true, "Fscan": true, "Fscanln": true,
	"Open": true, "Create": true, "OpenFile": true, "ReadFile": true, "WriteFile": true,
	"Stat": true, "Mkdir": true, "MkdirAll": true, "Remove": true, "RemoveAll": true, "Rename": true,
	"Getenv": true, "Setenv": true, "Getwd": true, "Chdir": true, "Exit": true, "Getpid": true,
	"ReadDir": true, "Readlink": true, "TempDir": true, "UserHomeDir": true,
	"Read": true, "ReadAll": true, "Write": true, "Copy": true, "CopyN": true,
	"ReadFull": true, "WriteString": true, "ReadAtLeast": true, "LimitReader": true,
	"NewWriter": true, "NewReadWriter": true,
	"Join": true, "Split": true, "SplitN": true, "Contains": true, "ContainsAny": true,
	"HasPrefix": true, "HasSuffix": true, "Replace": true, "ReplaceAll": true,
	"Trim": true, "TrimSpace": true, "TrimLeft": true, "TrimRight": true,
	"TrimPrefix": true, "TrimSuffix": true, "ToLower": true, "ToUpper": true,
	"ToTitle": true, "Repeat": true, "Index": true, "LastIndex": true,
	"Count": true, "Fields": true, "EqualFold": true, "NewReader": true, "NewReplacer": true,
	"Atoi": true, "Itoa": true, "ParseInt": true, "ParseUint": true, "ParseFloat": true,
	"FormatInt": true, "FormatUint": true, "FormatFloat": true, "Quote": true, "Unquote": true,
	"Marshal": true, "Unmarshal": true, "NewDecoder": true, "NewEncoder": true,
	"Encode": true, "Decode": true, "MarshalIndent": true, "Compact": true, "Indent": true,
	"Handle": true, "HandleFunc": true, "ListenAndServe": true, "ListenAndServeTLS": true,
	"NewRequest": true, "NewServeMux": true, "Redirect": true, "Error": true, "NotFound": true,
	"Get": true, "Post": true, "Head": true, "PostForm": true, "ReadRequest": true, "ReadResponse": true,
	"Now": true, "Since": true, "Until": true, "Sleep": true,
	"NewTicker": true, "NewTimer": true, "After": true, "AfterFunc": true,
	"Parse": true, "ParseDuration": true,
	"Wait": true, "Done": true, "Add": true, "Once": true,
	"Lock": true, "Unlock": true, "RLock": true, "RUnlock": true, "NewCond": true, "Pool": true,
	"Sort": true, "Slice": true, "SliceStable": true, "Search": true, "SearchInts": true,
	"Ints": true, "Float64s": true, "Strings": true, "Reverse": true, "IsSorted": true,
	"Abs": true, "Ceil": true, "Floor": true, "Round": true, "Max": true, "Min": true,
	"Pow": true, "Sqrt": true, "Sin": true, "Cos": true, "Tan": true, "Log": true, "Exp": true, "Mod": true,
	"Fatal": true, "Fatalf": true, "Fatalln": true,
	"Background": true, "TODO": true,
	"WithCancel": true, "WithDeadline": true, "WithTimeout": true, "WithValue": true,
}

func isKeyword(name string) bool {
	return goKeywords[name]
}
