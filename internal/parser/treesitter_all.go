package parser

import (
	"fmt"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	tscpp "github.com/smacker/go-tree-sitter/cpp"
	tscsharp "github.com/smacker/go-tree-sitter/csharp"
	tsjava "github.com/smacker/go-tree-sitter/java"
	tsjavascript "github.com/smacker/go-tree-sitter/javascript"
	tspython "github.com/smacker/go-tree-sitter/python"
	tsruby "github.com/smacker/go-tree-sitter/ruby"
	tsrust "github.com/smacker/go-tree-sitter/rust"

	"code-detector/internal/model"
)

// tsLangDef 定义一门语言在 tree-sitter 中的解析规则
type tsLangDef struct {
	Name       string
	Extensions []string
	GetLang    func() *sitter.Language
	// 函数/方法查询
	FuncQuery string // 捕获: name, body, func
	// 调用查询
	CallQuery    string // 捕获: callee
	SelCallQuery string // 捕获: callee (可选)
	// 包/命名空间查询
	PkgQuery string // 捕获: pkg (可选)
	// 全局变量查询
	VarQuery   string // 捕获: name, type(可选)
	ConstQuery string // 捕获: name, type(可选)
	// 关键字过滤（避免把关键字当函数调用）
	Keywords map[string]bool
}

// tsLangRegistry 语言配置注册表
var tsLangRegistry = []tsLangDef{
	{
		Name: "python", Extensions: []string{".py"},
		GetLang:    pyGetLang,
		FuncQuery:  `(function_definition name: (identifier) @name body: (block) @body) @func`,
		CallQuery:  `(call function: (identifier) @callee) @call`,
		PkgQuery:   `(module (import_statement (dotted_name (identifier) @pkg)))`,
		VarQuery:   ``,
		ConstQuery: ``,
	},
	{
		Name: "java", Extensions: []string{".java"},
		GetLang:    javaGetLang,
		FuncQuery:  `(method_declaration name: (identifier) @name body: (block) @body) @func`,
		CallQuery:  `(method_invocation name: (identifier) @callee) @call`,
		SelCallQuery: `(method_invocation object: (_) name: (identifier) @callee) @call`,
		PkgQuery:   `(package_declaration (scoped_identifier (identifier) @pkg))`,
	},
	{
		Name: "javascript", Extensions: []string{".js", ".jsx", ".mjs"},
		GetLang:    jsGetLang,
		FuncQuery:  `(function_declaration name: (identifier) @name body: (statement_block) @body) @func`,
		CallQuery:  `(call_expression function: (identifier) @callee) @call`,
		SelCallQuery: `(call_expression function: (member_expression property: (property_identifier) @callee)) @call`,
	},
	{
		Name: "typescript", Extensions: []string{".ts", ".tsx"},
		GetLang:    tsGetLang,
		FuncQuery:  `(function_declaration name: (identifier) @name body: (statement_block) @body) @func`,
		CallQuery:  `(call_expression function: (identifier) @callee) @call`,
		SelCallQuery: `(call_expression function: (member_expression property: (property_identifier) @callee)) @call`,
	},
	{
		Name: "cpp", Extensions: []string{".cpp", ".cxx", ".cc", ".c", ".h", ".hpp"},
		GetLang:    cppGetLang,
		FuncQuery:  `(function_definition declarator: (function_declarator declarator: (identifier) @name) body: (compound_statement) @body) @func`,
		CallQuery:  `(call_expression function: (identifier) @callee) @call`,
		SelCallQuery: `(call_expression function: (field_expression field: (field_identifier) @callee)) @call`,
	},
	{
		Name: "rust", Extensions: []string{".rs"},
		GetLang:    rustGetLang,
		FuncQuery:  `(function_item name: (identifier) @name body: (block) @body) @func`,
		CallQuery:  `(call_expression function: (identifier) @callee) @call`,
		SelCallQuery: `(call_expression function: (scoped_identifier name: (identifier) @callee)) @call`,
	},
	{
		Name: "ruby", Extensions: []string{".rb"},
		GetLang:    rubyGetLang,
		FuncQuery:  `(method name: (identifier) @name body: (body_statement) @body) @func`,
		CallQuery:  `(call method: (identifier) @callee) @call`,
		PkgQuery:   ``,
	},
	{
		Name: "csharp", Extensions: []string{".cs"},
		GetLang:    csGetLang,
		FuncQuery:  `(method_declaration name: (identifier) @name body: (block) @body) @func`,
		CallQuery:  `(invocation_expression function: (identifier) @callee) @call`,
		SelCallQuery: `(invocation_expression function: (member_access_expression name: (identifier) @callee)) @call`,
	},
}

// 各语言的 GetLang 包装函数
func pyGetLang() *sitter.Language   { return tspython.GetLanguage() }
func javaGetLang() *sitter.Language  { return tsjava.GetLanguage() }
func jsGetLang() *sitter.Language    { return tsjavascript.GetLanguage() }
func tsGetLang() *sitter.Language    { return tsjavascript.GetLanguage() }
func cppGetLang() *sitter.Language   { return tscpp.GetLanguage() }
func rustGetLang() *sitter.Language  { return tsrust.GetLanguage() }
func rubyGetLang() *sitter.Language  { return tsruby.GetLanguage() }
func csGetLang() *sitter.Language    { return tscsharp.GetLanguage() }

// getLangExt 通过扩展名查找语言定义
func getLangDef(ext string) *tsLangDef {
	for i := range tsLangRegistry {
		for _, e := range tsLangRegistry[i].Extensions {
			if e == ext {
				return &tsLangRegistry[i]
			}
		}
	}
	return nil
}

// ─── TreeSitterParser ────────────────────────────────

// TreeSitterParser 通用 tree-sitter 解析器，通过语言配置驱动
type TreeSitterParser struct {
	def *tsLangDef
}

// NewTreeSitterParser 根据扩展名创建对应的 tree-sitter 解析器
func NewTreeSitterParser(ext string) *TreeSitterParser {
	def := getLangDef(ext)
	if def == nil {
		return nil
	}
	return &TreeSitterParser{def: def}
}

func (p *TreeSitterParser) Language() string { return p.def.Name }

func (p *TreeSitterParser) Parse(filePath string, content []byte) ([]*model.Function, error) {
	def := p.def
	lang := def.GetLang()
	root, err := tsParseRootFor(content, lang)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filePath, err)
	}

	pkgName := tsFirstFor(root, def.PkgQuery, "pkg", content, lang)
	results := make([]*model.Function, 0)

	tsEachFor(root, def.FuncQuery, content, lang, func(name, body, fullText string) {
		if name == "" || fullText == "" {
			return
		}
		lineStart, lineEnd := tsFindLineFor(name, root, def.FuncQuery, content, lang)
		bodyNode := tsFindBodyFor(name, root, def.FuncQuery, content, lang)
		stats := tsAnalyzeCallsFor(bodyNode, content, lang, def.CallQuery, def.SelCallQuery, def.Keywords)
		results = append(results, &model.Function{
			Name: name, PackageName: pkgName, Language: def.Name,
			FilePath: filepath.ToSlash(filePath),
			LineStart: lineStart, LineEnd: lineEnd, Body: fullText,
			Dependencies: stats.Callees, CallCount: stats.CallCount,
			NestingDepth: stats.NestingDepth,
		})
	})
	return results, nil
}

func (p *TreeSitterParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	// 大部分语言暂不支持 tree-sitter 全局变量提取（TODO）
	return nil, nil
}

// ─── 通用树操作 ──────────────────────────────────────

func tsParseRootFor(content []byte, lang *sitter.Language) (*sitter.Node, error) {
	p := sitter.NewParser()
	if p == nil {
		return nil, fmt.Errorf("NewParser failed")
	}
	p.SetLanguage(lang)
	tree, err := p.ParseCtx(tsCtx, nil, content)
	if err != nil || tree == nil {
		return nil, fmt.Errorf("parse: %v", err)
	}
	root := tree.RootNode()
	if root == nil {
		tree.Close()
		return nil, fmt.Errorf("nil root")
	}
	return root, nil
}

func tsEachFor(root *sitter.Node, queryStr string, content []byte, lang *sitter.Language, fn func(name, body, fullText string)) {
	if queryStr == "" {
		return
	}
	q, err := sitter.NewQuery([]byte(queryStr), lang)
	if err != nil || q == nil {
		return
	}
	defer q.Close()
	cursor := sitter.NewQueryCursor()
	if cursor == nil {
		return
	}
	defer cursor.Close()
	cursor.Exec(q, root)
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}
		var name, body, fullText string
		for _, c := range m.Captures {
			if c.Node == nil {
				continue
			}
			switch q.CaptureNameForId(c.Index) {
			case "name":
				name = strings.TrimSpace(c.Node.Content(content))
			case "body":
				body = c.Node.Content(content)
			case "func":
				fullText = c.Node.Content(content)
			}
		}
		if name != "" && fullText != "" {
			fn(name, body, fullText)
		}
	}
}

func tsFirstFor(root *sitter.Node, queryStr, capName string, content []byte, lang *sitter.Language) string {
	if queryStr == "" {
		return ""
	}
	q, err := sitter.NewQuery([]byte(queryStr), lang)
	if err != nil || q == nil {
		return ""
	}
	defer q.Close()
	cursor := sitter.NewQueryCursor()
	if cursor == nil {
		return ""
	}
	defer cursor.Close()
	cursor.Exec(q, root)
	m, ok := cursor.NextMatch()
	if !ok {
		return ""
	}
	for _, c := range m.Captures {
		if q.CaptureNameForId(c.Index) == capName && c.Node != nil {
			return strings.TrimSpace(c.Node.Content(content))
		}
	}
	return ""
}

func tsFindLineFor(name string, root *sitter.Node, queryStr string, content []byte, lang *sitter.Language) (int, int) {
	if queryStr == "" {
		return 0, 0
	}
	q, err := sitter.NewQuery([]byte(queryStr), lang)
	if err != nil || q == nil {
		return 0, 0
	}
	defer q.Close()
	cursor := sitter.NewQueryCursor()
	if cursor == nil {
		return 0, 0
	}
	defer cursor.Close()
	cursor.Exec(q, root)
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}
		var foundName string
		var funcNode *sitter.Node
		for _, c := range m.Captures {
			switch q.CaptureNameForId(c.Index) {
			case "name":
				if c.Node != nil {
					foundName = strings.TrimSpace(c.Node.Content(content))
				}
			case "func":
				funcNode = c.Node
			}
		}
		if foundName == name && funcNode != nil {
			return int(funcNode.StartPoint().Row) + 1, int(funcNode.EndPoint().Row) + 1
		}
	}
	return 0, 0
}

func tsFindBodyFor(name string, root *sitter.Node, queryStr string, content []byte, lang *sitter.Language) *sitter.Node {
	if queryStr == "" {
		return nil
	}
	q, err := sitter.NewQuery([]byte(queryStr), lang)
	if err != nil || q == nil {
		return nil
	}
	defer q.Close()
	cursor := sitter.NewQueryCursor()
	if cursor == nil {
		return nil
	}
	defer cursor.Close()
	cursor.Exec(q, root)
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}
		var foundName string
		var bodyNode *sitter.Node
		for _, c := range m.Captures {
			switch q.CaptureNameForId(c.Index) {
			case "name":
				if c.Node != nil {
					foundName = strings.TrimSpace(c.Node.Content(content))
				}
			case "body":
				bodyNode = c.Node
			}
		}
		if foundName == name {
			return bodyNode
		}
	}
	return nil
}

func tsAnalyzeCallsFor(bodyNode *sitter.Node, content []byte, lang *sitter.Language, callQuery, selCallQuery string, keywords map[string]bool) *model.CallStats {
	stats := &model.CallStats{}
	if bodyNode == nil || callQuery == "" {
		return stats
	}
	seen := make(map[string]bool)

	addCall := func(callee string) {
		if callee == "" {
			return
		}
		if keywords != nil && keywords[callee] {
			return
		}
		if !seen[callee] {
			stats.Callees = append(stats.Callees, callee)
			seen[callee] = true
		}
		stats.CallCount++
	}

	// 普通调用
	q, _ := sitter.NewQuery([]byte(callQuery), lang)
	if q != nil {
		defer q.Close()
		cursor := sitter.NewQueryCursor()
		if cursor != nil {
			defer cursor.Close()
			cursor.Exec(q, bodyNode)
			for {
				m, ok := cursor.NextMatch()
				if !ok {
					break
				}
				for _, c := range m.Captures {
					if q.CaptureNameForId(c.Index) == "callee" && c.Node != nil {
						addCall(strings.TrimSpace(c.Node.Content(content)))
					}
				}
			}
		}
	}

	// selector 调用
	if selCallQuery != "" {
		q2, _ := sitter.NewQuery([]byte(selCallQuery), lang)
		if q2 != nil {
			defer q2.Close()
			cursor2 := sitter.NewQueryCursor()
			if cursor2 != nil {
				defer cursor2.Close()
				cursor2.Exec(q2, bodyNode)
				for {
					m, ok := cursor2.NextMatch()
					if !ok {
						break
					}
					for _, c := range m.Captures {
						if q2.CaptureNameForId(c.Index) == "callee" && c.Node != nil {
							addCall(strings.TrimSpace(c.Node.Content(content)))
						}
					}
				}
			}
		}
	}

	stats.NestingDepth = tsNestingDepth(bodyNode)
	return stats
}

func tsNestingDepth(node *sitter.Node) int {
	if node == nil {
		return 0
	}
	maxDepth := 0
	var walk func(n *sitter.Node, depth int)
	walk = func(n *sitter.Node, depth int) {
		if n == nil {
			return
		}
		if n.Type() == "call_expression" || n.Type() == "call" || n.Type() == "method_invocation" || n.Type() == "invocation_expression" {
			if depth > maxDepth {
				maxDepth = depth
			}
			for i := 0; i < int(n.ChildCount()); i++ {
				walk(n.Child(i), depth+1)
			}
		} else {
			for i := 0; i < int(n.ChildCount()); i++ {
				walk(n.Child(i), depth)
			}
		}
	}
	walk(node, 0)
	return maxDepth
}
