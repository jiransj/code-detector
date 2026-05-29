package parser

import (
	"regexp"
	"strings"

	"code-detector/internal/model"
)

// RubyParser 解析 Ruby 源文件
type RubyParser struct{}

func NewRubyParser() *RubyParser { return &RubyParser{} }
func (p *RubyParser) Language() string { return "ruby" }

func (p *RubyParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	text := string(content)
	lines := strings.Split(text, "\n")
	commentMask := makeCommentMask(lines, []string{"#"}, [][2]string{})

	var vars []*model.GlobalVariable
	rubyGlobalRegex := regexp.MustCompile(`^\s*(?:(?P<name>\$\w+)|(?P<const>[A-Z]\w+))\s*=\s*`)

	for i, line := range lines {
		if commentMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "def ") || strings.HasPrefix(trimmed, "class ") ||
			strings.HasPrefix(trimmed, "module ") || strings.HasPrefix(trimmed, "if ") ||
			strings.HasPrefix(trimmed, "unless ") || strings.HasPrefix(trimmed, "end") {
			continue
		}
		matches := rubyGlobalRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := rubyGlobalRegex.SubexpIndex("name")
		constIdx := rubyGlobalRegex.SubexpIndex("const")
		name := matches[nameIdx]
		if name == "" {
			name = matches[constIdx]
		}
		if name != "" {
			isConst := matches[constIdx] != ""
			vars = append(vars, &model.GlobalVariable{
				Name: name, VarType: "unknown", Language: "ruby",
				FilePath: filePath, LineNum: i + 1, IsConst: isConst,
			})
		}
	}
	return vars, nil
}

var rubyFuncRegex = regexp.MustCompile(
	`^\s*(?:def\s+)(?P<name>\w+(?:[?!])?)\s*[\(;]`,
)
var rubyCallRegex = regexp.MustCompile(`(?:(\w+)\.)?(\w+)\s*[\( ]`)

func (p *RubyParser) Parse(filePath string, content []byte) ([]*model.Function, error) {
	text := string(content)
	lines := strings.Split(text, "\n")

	// Ruby 注释 #，没有块注释（=begin/=end 极少用）
	commentMask := makeCommentMask(lines, []string{"#"}, [][2]string{})
	stringMask := makeStringMask(lines)

	type rbFuncStart struct {
		lineIdx int
		name    string
		indent  int
	}
	var starts []rbFuncStart

	for i, line := range lines {
		if commentMask[i] || stringMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		matches := rubyFuncRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := rubyFuncRegex.SubexpIndex("name")
		if nameIdx < 0 || nameIdx >= len(matches) {
			continue
		}
		name := matches[nameIdx]
		if name == "" {
			continue
		}

		indent := countIndent(line)
		starts = append(starts, rbFuncStart{lineIdx: i, name: name, indent: indent})
	}

	if len(starts) == 0 {
		return nil, nil
	}

	allFuncs := make([]*model.Function, 0, len(starts))
	for _, fs := range starts {
		bodyStart := fs.lineIdx + 1
		if bodyStart >= len(lines) {
			continue
		}

		bodyEnd := len(lines) - 1
		foundEnd := false
		for j := bodyStart; j < len(lines); j++ {
			if commentMask[j] || stringMask[j] {
				continue
			}
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "" {
				continue
			}
			if trimmed == "end" {
				bodyEnd = j
				foundEnd = true
				break
			}
		}
		if !foundEnd {
			continue
		}

		bodyLines := lines[fs.lineIdx : bodyEnd+1]
		body := strings.Join(bodyLines, "\n")

		callStats := extractCallStatsSimple(body, rubyCallRegex, isRubyKeyword)

		f := &model.Function{
			Name:         fs.name,
			Language:     "ruby",
			FilePath:     filePath,
			LineStart:    fs.lineIdx + 1,
			LineEnd:      bodyEnd + 1,
			Body:         body,
			Dependencies: callStats.Callees,
			CallCount:    callStats.CallCount,
			NestingDepth: callStats.NestingDepth,
		}
		allFuncs = append(allFuncs, f)
	}

	return allFuncs, nil
}

func extractRubyCalls(body string, callRegex *regexp.Regexp) []string {
	return extractCallsSimple(body, callRegex, isRubyKeyword)
}

func isRubyKeyword(name string) bool {
	switch name {
	case "if", "elsif", "else", "unless", "case", "when",
		"for", "while", "until", "do", "end", "begin",
		"rescue", "ensure", "raise", "throw", "catch",
		"return", "break", "next", "redo", "retry",
		"yield", "def", "class", "module", "lambda",
		"self", "super", "nil", "true", "false",
		"and", "or", "not", "in", "is_a?", "respond_to?",
		"puts", "print", "require", "include", "extend",
		"attr_reader", "attr_writer", "attr_accessor",
		"alias_method", "private", "protected", "public",
		"new", "initialize", "block_given?", "proc",
		"loop", "tap", "then", "each", "map", "select",
		"reject", "reduce", "inject", "sort", "sort_by",
		"find", "find_all", "collect", "flatten", "compact",
		"uniq", "merge", "keys", "values", "has_key?",
		"empty?", "nil?", "to_s", "to_i", "to_f", "to_a",
		"to_h", "inspect", "instance_of?",
		"kind_of?", "methods", "instance_methods",
		"send", "public_send",
		"fail", "warn", "abort", "exit":
		return true
	}
	return false
}

func isRubyStdFunction(name string) bool {
	switch name {
	case "if", "elsif", "else", "unless", "case", "when",
		"for", "while", "until", "do", "end", "begin",
		"rescue", "ensure", "raise", "throw", "catch",
		"return", "break", "next", "redo", "retry",
		"yield", "def", "class", "module", "lambda",
		"self", "super", "nil", "true", "false",
		"and", "or", "not", "in", "is_a?", "respond_to?",
		"puts", "print", "require", "include", "extend",
		"attr_reader", "attr_writer", "attr_accessor",
		"alias_method", "private", "protected", "public",
		"new", "initialize", "block_given?", "proc",
		"loop", "tap", "then", "each", "map", "select",
		"reject", "reduce", "inject", "sort", "sort_by",
		"find", "find_all", "collect", "flatten", "compact",
		"uniq", "merge", "keys", "values", "has_key?",
		"empty?", "nil?", "to_s", "to_i", "to_f", "to_a",
		"to_h", "inspect", "instance_of?",
		"kind_of?", "methods", "instance_methods",
		"send", "public_send",
		"fail", "warn", "abort", "exit",
		// Enumerable
		"all?", "any?", "chunk", "chunk_while",
		"collect_concat", "count", "cycle", "detect",
		"drop", "drop_while", "each_cons", "each_entry",
		"each_slice", "each_with_index", "each_with_object",
		"entries", "first", "grep", "grep_v",
		"group_by", "include?", "lazy", "max_by",
		"member?", "min_by", "minmax", "minmax_by",
		"none?", "one?", "partition", "reverse_each",
		"slice_after", "slice_before", "slice_when",
		"take", "take_while", "uniq_by", "zip",
		// Enumerator（与 keyword 段不重复的新增）
		"with_index", "with_object", "with_proc",
		"peek", "feed":
		return true
	}
	return false
}
