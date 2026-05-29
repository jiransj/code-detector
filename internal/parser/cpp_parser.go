package parser

import (
	"regexp"
	"strings"
	"unicode"

	"code-detector/internal/model"
)

// CPPParser 解析 C/C++ 源文件
type CPPParser struct{}

func NewCPPParser() *CPPParser { return &CPPParser{} }
func (p *CPPParser) Language() string { return "cpp" }

func (p *CPPParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	text := string(content)
	lines := strings.Split(text, "\n")
	commentMask := makeCommentMask(lines, []string{"//"}, [][2]string{{"/*", "*/"}})
	preprocMask := makePreprocessorMask(lines)

	var vars []*model.GlobalVariable
	// 匹配文件作用域的变量声明: type name = value; 或 type name;
	// 排除函数定义、返回类型声明等
	cppVarRegex := regexp.MustCompile(`^\s*(?:extern\s+)?(?:(?P<type>(?:unsigned\s+)?(?:long\s+)?(?:short\s+)?(?:signed\s+)?\w+(?:\s*\*)?(?:\s+const)?))\s+(?P<name>\w+)\s*(?:=|;)`)

	for i, line := range lines {
		if commentMask[i] || preprocMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		// 跳过函数定义、控制流
		if strings.HasPrefix(trimmed, "if ") || strings.HasPrefix(trimmed, "for ") ||
			strings.HasPrefix(trimmed, "while ") || strings.HasPrefix(trimmed, "switch ") {
			continue
		}
		matches := cppVarRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := cppVarRegex.SubexpIndex("name")
		typeIdx := cppVarRegex.SubexpIndex("type")
		name := matches[nameIdx]
		varType := ""
		if typeIdx >= 0 && typeIdx < len(matches) {
			varType = matches[typeIdx]
		}
		if name != "" && varType != "" && !isCPPKeyword(name) && !isCPPKeyword(varType) && (unicode.IsLetter(rune(name[0])) || name[0] == '_') {
			// 提取 namespace 和可见性
			pkgName := extractCPPNamespace(lines, commentMask)
			visibility := "public"
			if strings.Contains(trimmed, "static ") || strings.HasPrefix(trimmed, "static") {
				visibility = "private"
			}
			vars = append(vars, &model.GlobalVariable{
				Name: name, VarType: varType, Language: "cpp",
				PackageName: pkgName, Visibility: visibility,
				FilePath: filePath, LineNum: i + 1,
			})
		}
	}
	return vars, nil
}

// cppNamespaceRegex 匹配 C++ namespace 声明
var cppNamespaceRegex = regexp.MustCompile(`^\s*namespace\s+(?P<name>\w+)`)

// extractCPPNamespace 提取 C++ 文件中的第一个 namespace
func extractCPPNamespace(lines []string, commentMask []bool) string {
	for i, line := range lines {
		if commentMask[i] {
			continue
		}
		matches := cppNamespaceRegex.FindStringSubmatch(line)
		if matches != nil {
			nameIdx := cppNamespaceRegex.SubexpIndex("name")
			if nameIdx >= 0 && nameIdx < len(matches) {
				return matches[nameIdx]
			}
		}
	}
	return ""
}

var cppFuncRegex = regexp.MustCompile(
	`(?:(?:\w+(?:\[\])*(?:\s*<[^>]+>)?\s+)+)(?P<name>\w+)\s*\(`,
)

var cppCallRegex = regexp.MustCompile(`(?:(\w+)::)?(\w+)\s*\(`)

func (p *CPPParser) Parse(filePath string, content []byte) ([]*model.Function, error) {
	fl := NewFileLinesFromBytes(content)
	lines := fl.Lines()
	text := fl.Text()

	commentMask := makeCommentMask(lines, []string{"//"}, [][2]string{{"/*", "*/"}})
	stringMask := makeStringMask(lines)
	// 预处理掩码：跳过 #if 0 死代码、#define 宏定义体、#include 等
	preprocMask := makePreprocessorMask(lines)

	type cppFuncStart struct {
		lineIdx int
		name    string
	}
	var starts []cppFuncStart

	for i, line := range lines {
		if commentMask[i] || stringMask[i] || preprocMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// 排除 # 预处理指令（未被子掩码覆盖的）
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		matches := cppFuncRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := cppFuncRegex.SubexpIndex("name")
		if nameIdx < 0 || nameIdx >= len(matches) {
			continue
		}
		name := matches[nameIdx]
		if name == "" || isCPPKeyword(name) {
			continue
		}

		starts = append(starts, cppFuncStart{lineIdx: i, name: name})
	}

	if len(starts) == 0 {
		return nil, nil
	}

	allFuncs := make([]*model.Function, 0, len(starts))
	for _, fs := range starts {
		braceOffset := -1
		for j := fs.lineIdx; j < len(lines); j++ {
			line := lines[j]
			idx := findBraceInLine(line)
			if idx >= 0 && !commentMask[j] && !preprocMask[j] {
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
		endLine := fl.LineFromOffset( closeOffset)

		bodyStart := fl.LineOffset( startLine)
		bodyEnd := closeOffset + 1
		if bodyEnd > len(text) {
			bodyEnd = len(text)
		}
		body := text[bodyStart:bodyEnd]

		callStats := extractCallStats(body, cppCallRegex, stringMask, commentMask, startLine, endLine, isCPPKeyword, nil)

		f := &model.Function{
			Name:         fs.name,
			PackageName:  extractCPPNamespace(lines, commentMask),
			Language:     "cpp",
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

// cppKeywords 是 C/C++ 关键字和常用 STL 名的集合（map 数据驱动）
var cppKeywords = map[string]bool{
	"if": true, "else": true, "for": true, "while": true, "do": true, "switch": true, "case": true,
	"return": true, "break": true, "continue": true, "goto": true, "default": true,
	"try": true, "catch": true, "throw": true, "new": true, "delete": true, "this": true,
	"int": true, "long": true, "short": true, "char": true, "float": true, "double": true,
	"void": true, "bool": true, "signed": true, "unsigned": true, "const": true, "static": true,
	"virtual": true, "explicit": true, "inline": true, "typedef": true, "struct": true,
	"class": true, "enum": true, "union": true, "namespace": true, "using": true,
	"public": true, "private": true, "protected": true, "template": true, "typename": true,
	"sizeof": true, "nullptr": true, "true": true, "false": true, "auto": true, "register": true,
	"volatile": true, "mutable": true, "friend": true, "override": true, "final": true,
	"constexpr": true, "noexcept": true, "decltype": true, "nullptr_t": true,
	"vector": true, "list": true, "deque": true, "array": true, "forward_list": true,
	"map": true, "set": true, "multimap": true, "multiset": true,
	"unordered_map": true, "unordered_set": true,
	"stack": true, "queue": true, "priority_queue": true,
	"pair": true, "tuple": true, "optional": true, "variant": true, "any": true,
	"sort": true, "stable_sort": true, "partial_sort": true, "nth_element": true,
	"find": true, "find_if": true, "binary_search": true, "lower_bound": true, "upper_bound": true,
	"count": true, "count_if": true, "copy": true, "copy_if": true, "move": true,
	"transform": true, "replace": true, "replace_if": true, "remove": true, "remove_if": true,
	"unique": true, "reverse": true, "rotate": true, "shuffle": true, "random_shuffle": true,
	"fill": true, "generate": true, "for_each": true, "accumulate": true,
	"max": true, "min": true, "max_element": true, "min_element": true, "clamp": true,
	"all_of": true, "any_of": true, "none_of": true, "equal": true, "mismatch": true,
	"merge": true, "includes": true, "set_union": true, "set_intersection": true,
	"iota": true, "reduce": true, "transform_reduce": true,
	"cin": true, "cout": true, "cerr": true, "clog": true, "endl": true, "flush": true,
	"ifstream": true, "ofstream": true, "fstream": true,
	"stringstream": true, "istringstream": true, "ostringstream": true,
	"string": true, "wstring": true, "getline": true, "to_string": true,
	"stoi": true, "stol": true, "stoll": true, "stof": true, "stod": true, "stold": true, "to_wstring": true,
	"shared_ptr": true, "unique_ptr": true, "weak_ptr": true,
	"make_shared": true, "make_unique": true,
	"allocate_shared": true, "dynamic_pointer_cast": true, "static_pointer_cast": true,
	"thread": true, "mutex": true, "lock_guard": true, "unique_lock": true, "shared_lock": true,
	"condition_variable": true, "future": true, "promise": true, "async": true, "packaged_task": true,
	"this_thread": true, "sleep_for": true, "sleep_until": true, "yield": true,
	"abs": true, "fabs": true, "ceil": true, "floor": true, "round": true, "trunc": true,
	"sqrt": true, "cbrt": true, "hypot": true, "pow": true, "exp": true,
	"log": true, "log2": true, "log10": true,
	"sin": true, "cos": true, "tan": true, "asin": true, "acos": true, "atan": true, "atan2": true,
	"sinh": true, "cosh": true, "tanh": true, "fmod": true, "remainder": true,
}

func isCPPKeyword(name string) bool {
	return cppKeywords[name]
}

func filterCPPKeywords(deps []string) []string {
	filtered := make([]string, 0, len(deps))
	for _, d := range deps {
		if !isCPPKeyword(d) {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// makePreprocessorMask 标记 C/C++ 预处理指令行和死代码块
// 1. #if 0 ... #endif — 条件编译死代码
// 2. #define 宏定义及其续行体 — 宏展开不在语义层面可见
// 3. 其他 # 指令行直接跳过（由调用方处理）
func makePreprocessorMask(lines []string) []bool {
	mask := make([]bool, len(lines))
	inIfZero := false    // 在 #if 0 ... #endif 块内
	inDefine := false    // 在 #define 续行内
	ifZeroNest := 0      // 嵌套 #if/#ifdef/#ifndef 计数

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 处理多行宏定义续行
		if inDefine {
			mask[i] = true
			// 检查续行是否结束（行尾没有 \）
			if !strings.HasSuffix(strings.TrimRight(line, " \t\r"), "\\") {
				inDefine = false
			}
			continue
		}

		// 处理 #if 0 ... #endif 块
		if inIfZero {
			mask[i] = true
			if strings.HasPrefix(trimmed, "#if") || strings.HasPrefix(trimmed, "#ifdef") || strings.HasPrefix(trimmed, "#ifndef") {
				ifZeroNest++
			} else if strings.HasPrefix(trimmed, "#endif") {
				if ifZeroNest > 0 {
					ifZeroNest--
				} else {
					inIfZero = false
				}
			}
			continue
		}

		// 检测 # 预处理指令
		if strings.HasPrefix(trimmed, "#") {
			if trimmed == "#if 0" {
				// #if 0 — 死代码块开始
				inIfZero = true
				mask[i] = true
				ifZeroNest = 0
			} else if strings.HasPrefix(trimmed, "#if ") && strings.Contains(trimmed, "0") {
				// 可能还有其他 #if 0 变体
				parts := strings.Fields(trimmed)
				for _, p := range parts {
					if p == "0" {
						inIfZero = true
						mask[i] = true
						ifZeroNest = 0
						break
					}
				}
				mask[i] = true
			} else if strings.HasPrefix(trimmed, "#define") {
				// #define — 标记此行及所有续行
				mask[i] = true
				if strings.HasSuffix(strings.TrimRight(line, " \t\r"), "\\") {
					inDefine = true
				}
			} else {
				// 其他 # 指令（#include, #ifdef, #ifndef, #else, #elif, #pragma 等）
				mask[i] = true
			}
			continue
		}
	}

	return mask
}
