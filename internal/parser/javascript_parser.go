package parser

import (
	"regexp"
	"strings"

	"code-detector/internal/model"
)

// JSParser - parse  JavaScript/TypeScript 
type JSParser struct {
	language string // "javascript"  "typescript"
}

func NewJavascriptParser() *JSParser { return &JSParser{language: "javascript"} }
func NewTypescriptParser() *JSParser { return &JSParser{language: "typescript"} }

func (p *JSParser) Language() string { return p.language }

// extractJSModuleName  JS extract 
//  package.json  import/export 
func extractJSModuleName(filePath string, content []byte) string {
	text := string(content)
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "export default class ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 4 {
				return parts[3]
			}
		}
		if strings.HasPrefix(trimmed, "export default function ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 4 {
				return parts[3]
			}
		}
	}
	return extractModuleNameFromPath(filePath)
}

// extractModuleNameFromPath 
func extractModuleNameFromPath(filePath string) string {
	parts := strings.Split(strings.ReplaceAll(filePath, "\\", "/"), "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return "main"
}

// Parse -- parse  JS/TS 
func (p *JSParser) Parse(filePath string, content []byte) ([]*model.Function, error) {
	fl := NewFileLinesFromBytes(content)
	lines := fl.Lines()
	text := fl.Text()

	commentMask := makeJSCommentMask(lines)
	stringMask := makeJSStringMask(lines)

	// function name() / name = function() / name = () => / async function name()
	funcDefRegex := regexp.MustCompile(`(?:async\s+)?function\s+(?P<name>\w+)\s*\(|\b(\w+)\s*[:=]\s*(?:async\s+)?function\s*\(|\b(\w+)\s*[:=]\s*\([^)]*\)\s*=>`)

	type jsFuncStart struct {
		lineIdx int
		name    string
	}
	var starts []jsFuncStart

	for i, line := range lines {
		if commentMask[i] || stringMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		matches := funcDefRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		name := ""
		for _, m := range matches[1:] {
			if m != "" {
				name = m
				break
			}
		}
		if name == "" || name == "if" || name == "else" || name == "for" || name == "while" || name == "switch" || name == "catch" || name == "then" {
			continue
		}

		starts = append(starts, jsFuncStart{lineIdx: i, name: name})
	}

	if len(starts) == 0 {
		return nil, nil
	}

	moduleName := extractJSModuleName(filePath, content)
	allFuncs := make([]*model.Function, 0, len(starts))

	for _, fs := range starts {
		offset := fl.LineOffset(fs.lineIdx)
		line := lines[fs.lineIdx]
		braceIdx := findBraceInLine(line)
		if braceIdx < 0 {
			for j := fs.lineIdx + 1; j < len(lines); j++ {
				if commentMask[j] || stringMask[j] {
					continue
				}
				if idx := findBraceInLine(lines[j]); idx >= 0 {
					braceIdx = idx
					offset = fl.LineOffset(j) + idx
					break
				}
			}
			if braceIdx < 0 {
				continue
			}
		} else {
			offset += braceIdx
		}

		closeOffset, err := matchBrace(text, offset)
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

		callStats := extractCallStatsSimple(body, jsCallRegex, isJSStdFunction)

		f := &model.Function{
			Name:         fs.name,
			PackageName:  moduleName,
			Language:     p.language,
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

// Globals extract  JS/TS 
func (p *JSParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	text := string(content)
	lines := strings.Split(text, "\n")
	commentMask := makeJSCommentMask(lines)
	stringMask := makeJSStringMask(lines)

	var vars []*model.GlobalVariable

	//  const/let/var 
	varDeclRegex := regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+(?P<name>\w+)\s*(?:=|;)`)

	for i, line := range lines {
		if commentMask[i] || stringMask[i] {
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
		if nameIdx < 0 || nameIdx >= len(matches) {
			continue
		}
		name := matches[nameIdx]
		if name == "" {
			continue
		}
		vars = append(vars, &model.GlobalVariable{
			Name:     name,
			Language: p.language,
			FilePath: filePath,
			LineNum:  i + 1,
			IsConst:  strings.Contains(trimmed, "const"),
		})
	}
	return vars, nil
}

// makeJSCommentMask  JS/TS 
func makeJSCommentMask(lines []string) []bool {
	mask := make([]bool, len(lines))
	inBlock := false
	inBlockEnd := -1

	for i, line := range lines {
		if inBlock {
			mask[i] = true
			if i <= inBlockEnd {
				continue
			}
			inBlock = false
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "//") {
			mask[i] = true
			continue
		}

		if strings.HasPrefix(trimmed, "/*") {
			mask[i] = true
			endIdx := strings.Index(trimmed, "*/")
			if endIdx < 0 {
				inBlock = true
				// 
				for j := i + 1; j < len(lines); j++ {
					if strings.Contains(lines[j], "*/") {
						inBlockEnd = j
						mask[j] = true
						break
					}
					mask[j] = true
				}
			}
			continue
		}

		if strings.Contains(trimmed, "//") {
			commentIdx := strings.Index(trimmed, "//")
			before := trimmed[:commentIdx]
			if !strings.Contains(before, "\"") && !strings.Contains(before, "'") && !strings.Contains(before, "`") {
				mask[i] = true
			}
		}
	}
	return mask
}

// makeJSStringMask  JS/TS 
func makeJSStringMask(lines []string) []bool {
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
			if ch == '\\' && (inDouble || inSingle || inBacktick) {
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

// isJSStmtKeyword  JavaScript 
func isJSStmtKeyword(name string) bool {
	switch name {
	case "if", "else", "for", "while", "do", "switch", "case",
		"try", "catch", "finally", "with", "import", "export",
		"class", "function", "async", "await", "yield", "return",
		"throw", "new", "debugger":
		return true
	}
	return false
}

// isJSKeyword  JS 
func isJSKeyword(name string) bool {
	switch name {
	case "if", "else", "for", "while", "do",
		"switch", "case", "default", "break",
		"continue", "return", "throw", "try",
		"catch", "finally", "new", "this",
		"super", "typeof", "instanceof", "void",
		"delete", "in", "of", "yield", "await",
		"async", "function", "class", "extends",
		"import", "export", "from", "const",
		"let", "var", "null", "undefined",
		"true", "false", "NaN", "Infinity":
		return true
	}
	return false
}

func filterJSKeywords(deps []string) []string {
	filtered := make([]string, 0, len(deps))
	for _, d := range deps {
		if !isJSKeyword(d) {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// jsCallRegex  JavaScript  qualified call 
var jsCallRegex = regexp.MustCompile(`(?:(\w+)\.)?(\w+)\s*\(`)

// jsStdFunctions  JavaScript/TypeScript /API map 
var jsStdFunctions = map[string]bool{
	"if": true, "else": true, "for": true, "while": true, "do": true,
	"switch": true, "case": true, "default": true, "break": true,
	"continue": true, "return": true, "throw": true, "try": true,
	"catch": true, "finally": true, "new": true, "this": true,
	"super": true, "typeof": true, "instanceof": true, "void": true,
	"delete": true, "in": true, "of": true, "yield": true, "await": true,
	"async": true, "function": true, "class": true, "extends": true,
	"import": true, "export": true, "from": true, "const": true,
	"let": true, "var": true, "null": true, "undefined": true,
	"true": true, "false": true, "NaN": true, "Infinity": true,
	"console": true, "require": true, "define": true, "module": true, "exports": true,
	"String": true, "Number": true, "Boolean": true, "Array": true,
	"Object": true, "Function": true, "Date": true, "RegExp": true,
	"Map": true, "Set": true, "Promise": true, "Symbol": true, "WeakMap": true, "WeakSet": true,
	"parseInt": true, "parseFloat": true,
	"JSON": true, "Math": true, "Reflect": true, "Proxy": true,
	"setTimeout": true, "setInterval": true, "clearTimeout": true,
	"clearInterval": true, "setImmediate": true, "clearImmediate": true,
	"map": true, "filter": true, "reduce": true, "reduceRight": true, "forEach": true,
	"find": true, "findIndex": true, "some": true, "every": true, "includes": true,
	"push": true, "pop": true, "shift": true, "unshift": true, "slice": true, "splice": true,
	"concat": true, "join": true, "indexOf": true, "lastIndexOf": true, "sort": true,
	"reverse": true, "fill": true, "flat": true, "flatMap": true,
	"charAt": true, "charCodeAt": true, "match": true, "matchAll": true,
	"replace": true, "replaceAll": true, "search": true,
	"split": true, "substring": true, "toLowerCase": true, "toUpperCase": true,
	"trim": true, "trimStart": true, "trimEnd": true, "padStart": true, "padEnd": true,
	"startsWith": true, "endsWith": true, "repeat": true,
	"keys": true, "values": true, "entries": true, "assign": true, "create": true,
	"defineProperty": true, "defineProperties": true, "freeze": true,
	"getOwnPropertyDescriptor": true, "getOwnPropertyNames": true,
	"getPrototypeOf": true, "is": true, "isFrozen": true, "isSealed": true,
	"preventExtensions": true, "seal": true, "setPrototypeOf": true,
	"all": true, "race": true, "allSettled": true, "any": true, "resolve": true, "reject": true, "then": true,
	"abs": true, "ceil": true, "floor": true, "round": true, "max": true, "min": true, "pow": true,
	"sqrt": true, "random": true, "trunc": true, "sign": true, "cbrt": true,
	"sin": true, "cos": true, "tan": true, "asin": true, "acos": true, "atan": true, "atan2": true,
	"exp": true, "log": true, "log2": true, "log10": true,
	"fetch": true, "decodeURI": true, "decodeURIComponent": true,
	"encodeURI": true, "encodeURIComponent": true, "escape": true, "unescape": true,
	"isFinite": true, "isNaN": true,
	"document": true, "querySelector": true, "querySelectorAll": true,
	"getElementById": true, "createElement": true, "addEventListener": true,
	"removeEventListener": true, "localStorage": true, "sessionStorage": true,
	"setItem": true, "getItem": true, "removeItem": true, "clear": true,
	"process": true, "cwd": true, "chdir": true, "env": true, "argv": true, "exit": true,
	"Buffer": true, "alloc": true,
}

func isJSStdFunction(name string) bool {
	return jsStdFunctions[name]
}
