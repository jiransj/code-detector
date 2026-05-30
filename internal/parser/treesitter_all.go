package parser

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	tscpp "github.com/smacker/go-tree-sitter/cpp"
	tscsharp "github.com/smacker/go-tree-sitter/csharp"
	tsembedded "code-detector/internal/parser/erb"
	tsjava "github.com/smacker/go-tree-sitter/java"
	tsjavascript "github.com/smacker/go-tree-sitter/javascript"
	tskotlin "github.com/smacker/go-tree-sitter/kotlin"
	tslua "github.com/smacker/go-tree-sitter/lua"
	tsphp "github.com/smacker/go-tree-sitter/php"
	tspython "github.com/smacker/go-tree-sitter/python"
	tsruby "github.com/smacker/go-tree-sitter/ruby"
	tsrust "github.com/smacker/go-tree-sitter/rust"
	tsscala "github.com/smacker/go-tree-sitter/scala"
	tsswift "github.com/smacker/go-tree-sitter/swift"
	tstypescript "github.com/smacker/go-tree-sitter/typescript/typescript"
	tstypescriptTsx "github.com/smacker/go-tree-sitter/typescript/tsx"

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
		VarQuery:   `(module (expression_statement (assignment left: (identifier) @name type: (_)? @type)) @decl)`,
		ConstQuery: ``,
	},
	{
		Name: "java", Extensions: []string{".java"},
		GetLang:    javaGetLang,
		FuncQuery:  `(method_declaration name: (identifier) @name body: (block) @body) @func`,
		CallQuery:  `(method_invocation name: (identifier) @callee) @call`,
		SelCallQuery: `(method_invocation object: (_) name: (identifier) @callee) @call`,
		PkgQuery:   `(package_declaration (scoped_identifier (identifier) @pkg))`,
		// 注意：静态字段判断需要语义分析，此处只捕获所有顶层字段声明
		VarQuery:   `(program (field_declaration declarator: (variable_declarator name: (identifier) @name)) @decl)`,
		ConstQuery: ``,
	},
	{
		Name: "javascript", Extensions: []string{".js", ".jsx", ".mjs"},
		GetLang:    jsGetLang,
		FuncQuery:  `(function_declaration name: (identifier) @name body: (statement_block) @body) @func`,
		CallQuery:  `(call_expression function: (identifier) @callee) @call`,
		SelCallQuery: `(call_expression function: (member_expression property: (property_identifier) @callee)) @call`,
		VarQuery:   `(program (lexical_declaration (variable_declarator name: (identifier) @name type: (_)? @type)) @decl)`,
		ConstQuery: `(program (lexical_declaration (variable_declarator name: (identifier) @name type: (_)? @type)) @decl)`,
	},
	{
		Name: "typescript", Extensions: []string{".ts", ".tsx"},
		GetLang:    tsGetLang,
		FuncQuery:  `(function_declaration name: (identifier) @name body: (statement_block) @body) @func`,
		CallQuery:  `(call_expression function: (identifier) @callee) @call`,
		SelCallQuery: `(call_expression function: (member_expression property: (property_identifier) @callee)) @call`,
		VarQuery:   `(program (lexical_declaration (variable_declarator name: (identifier) @name type: (_)? @type)) @decl)`,
		ConstQuery: `(program (lexical_declaration (variable_declarator name: (identifier) @name type: (_)? @type)) @decl)`,
	},
	{
		Name: "cpp", Extensions: []string{".cpp", ".cxx", ".cc", ".c", ".h", ".hpp"},
		GetLang:    cppGetLang,
		FuncQuery:  `(function_definition declarator: (function_declarator declarator: (identifier) @name) body: (compound_statement) @body) @func`,
		CallQuery:  `(call_expression function: (identifier) @callee) @call`,
		SelCallQuery: `(call_expression function: (field_expression field: (field_identifier) @callee)) @call`,
		// C/C++ 顶层声明: const/static 变量, 普通全局变量
		VarQuery:   `(translation_unit (declaration declarator: (identifier) @name type: (_)? @type) @decl)`,
		ConstQuery: `(translation_unit (declaration declarator: (identifier) @name type: (_)? @type) @decl)`,
	},
	{
		Name: "rust", Extensions: []string{".rs"},
		GetLang:    rustGetLang,
		FuncQuery:  `(function_item name: (identifier) @name body: (block) @body) @func`,
		CallQuery:  `(call_expression function: (identifier) @callee) @call`,
		SelCallQuery: `(call_expression function: (scoped_identifier name: (identifier) @callee)) @call`,
		VarQuery:   `(source_file (static_item name: (identifier) @name type: (_)? @type value: (_)? @value) @decl)`,
		ConstQuery: `(source_file (const_item name: (identifier) @name type: (_)? @type value: (_)? @value) @decl)`,
	},
	{
		Name: "embedded_template", Extensions: []string{".erb"},
		GetLang:    erbGetLang,
		// embedded_template 的 AST: directive (代码块)/output_directive (输出)/content (文本)
		// 捕获 directive 节点整体作为函数体，code 子节点作为 name 占位
		FuncQuery:  `(directive) @func`,
		CallQuery:  ``,
		PkgQuery:   ``,
		VarQuery:   ``,
		ConstQuery: ``,
	},
	{
		Name: "ruby", Extensions: []string{".rb"},
		GetLang:    rubyGetLang,
		FuncQuery:  `(method name: (identifier) @name body: (body_statement) @body) @func`,
		CallQuery:  `(call method: (identifier) @callee) @call`,
		PkgQuery:   ``,
		// Ruby 全局变量以 $ 开头，常量以大写字母开头
		VarQuery:   `(program (assignment left: (identifier) @name value: (_)? @value) @decl)`,
		ConstQuery: ``,
	},
	{
		Name: "csharp", Extensions: []string{".cs"},
		GetLang:    csGetLang,
		FuncQuery:  `(method_declaration name: (identifier) @name body: (block) @body) @func`,
		CallQuery:  `(invocation_expression function: (identifier) @callee) @call`,
		SelCallQuery: `(invocation_expression function: (member_access_expression name: (identifier) @callee)) @call`,
		VarQuery:   `(compilation_unit (field_declaration declarator: (variable_declarator name: (identifier) @name type: (_)? @type)) @decl)`,
		ConstQuery: `(compilation_unit (field_declaration declarator: (variable_declarator name: (identifier) @name type: (_)? @type)) @decl)`,
	},
	{
		Name: "typescript", Extensions: []string{".ts"},
		GetLang:    tsGetLang,
		FuncQuery:  `(function_declaration name: (identifier) @name body: (statement_block) @body) @func`,
		CallQuery:  `(call_expression function: (identifier) @callee) @call`,
		SelCallQuery: `(call_expression function: (member_expression property: (property_identifier) @callee)) @call`,
	},
	{
		Name: "tsx", Extensions: []string{".tsx"},
		GetLang:    tsxGetLang,
		FuncQuery:  `(function_declaration name: (identifier) @name body: (statement_block) @body) @func`,
		CallQuery:  `(call_expression function: (identifier) @callee) @call`,
		SelCallQuery: `(call_expression function: (member_expression property: (property_identifier) @callee)) @call`,
	},
	{
		Name: "swift", Extensions: []string{".swift"},
		GetLang:    swiftGetLang,
		FuncQuery:  `(function_declaration name: (simple_identifier) @name body: (function_body) @body) @func`,
		CallQuery:  `(call_expression function: (simple_identifier) @callee) @call`,
		SelCallQuery: `(call_expression function: (member_access_expression member: (simple_identifier) @callee)) @call`,
		// Swift: 顶层 var/let 声明
		VarQuery:   `(source_file (variable_declaration (pattern_binding name: (simple_identifier) @name type: (_)? @type)) @decl)`,
		ConstQuery: ``,
	},
	{
		Name: "kotlin", Extensions: []string{".kt", ".kts"},
		GetLang:    kotlinGetLang,
		FuncQuery:  `(function_declaration name: (simple_identifier) @name body: (function_body) @body) @func`,
		CallQuery:  `(call_expression function: (simple_identifier) @callee) @call`,
		SelCallQuery: `(call_expression function: (navigation_expression navigation_suffix: (simple_identifier) @callee)) @call`,
		// Kotlin: 顶层 var/val 属性
		VarQuery:   `(source_file (property_declaration (variable_declaration name: (simple_identifier) @name type: (_)? @type)) @decl)`,
		ConstQuery: ``,
	},
	{
		Name: "php", Extensions: []string{".php"},
		GetLang:    phpGetLang,
		FuncQuery:  `(function_definition name: (name) @name body: (body) @body) @func`,
		CallQuery:  `(function_call_expression function: (name) @callee) @call`,
		// PHP: 顶层 $变量 + const 声明
		VarQuery:   `(program (expression_statement (assignment left: (variable_name (name) @name) right: (_) @value)) @decl)`,
		ConstQuery: `(program (const_declaration (const_element name: (name) @name value: (_) @value)) @decl)`,
	},
	{
		Name: "lua", Extensions: []string{".lua"},
		GetLang:    luaGetLang,
		FuncQuery:  `(function_declaration name: (identifier) @name body: (block) @body) @func`,
		CallQuery:  `(function_call function: (identifier) @callee) @call`,
		SelCallQuery: `(function_call function: (dot_index_expression field: (identifier) @callee)) @call`,
		// Lua: 全局变量 = 模块级别的赋值
		VarQuery:   `(program (assignment_statement variable: (identifier) @name value: (_)? @value) @decl)`,
	},
	{
		Name: "scala", Extensions: []string{".scala"},
		GetLang:    scalaGetLang,
		FuncQuery:  `(function_definition name: (identifier) @name body: (block) @body) @func`,
		CallQuery:  `(call_expression function: (identifier) @callee) @call`,
		SelCallQuery: `(call_expression function: (selector_expression member: (identifier) @callee)) @call`,
		// Scala: val/var 定义
		VarQuery:   `(source_file (val_definition (identifier) @name type: (_)? @type value: (_)? @value) @decl)`,
		ConstQuery: ``,
	},
}

// 各语言的 GetLang 包装函数
func pyGetLang() *sitter.Language   { return tspython.GetLanguage() }
func javaGetLang() *sitter.Language  { return tsjava.GetLanguage() }
func jsGetLang() *sitter.Language    { return tsjavascript.GetLanguage() }
func cppGetLang() *sitter.Language   { return tscpp.GetLanguage() }
func rustGetLang() *sitter.Language  { return tsrust.GetLanguage() }
func rubyGetLang() *sitter.Language  { return tsruby.GetLanguage() }
func erbGetLang() *sitter.Language  { return tsembedded.GetLanguage() }
func csGetLang() *sitter.Language    { return tscsharp.GetLanguage() }
func tsGetLang() *sitter.Language    { return tstypescript.GetLanguage() }
func tsxGetLang() *sitter.Language   { return tstypescriptTsx.GetLanguage() }
func swiftGetLang() *sitter.Language  { return tsswift.GetLanguage() }
func kotlinGetLang() *sitter.Language { return tskotlin.GetLanguage() }
func phpGetLang() *sitter.Language   { return tsphp.GetLanguage() }
func luaGetLang() *sitter.Language   { return tslua.GetLanguage() }
func scalaGetLang() *sitter.Language { return tsscala.GetLanguage() }

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

// tsGlobalsFor 用 tree-sitter 查询提取单个文件中的全局变量
// queryStr 是 VarQuery 或 ConstQuery，要求捕获: @name, @type(可选), @value(可选), @decl
// topLevelType 是顶层节点的类型名（如 "source_file", "program", "module"）
func tsGlobalsFor(root *sitter.Node, queryStr string, content []byte, lang *sitter.Language, topLevelType string) []globVar {
	if queryStr == "" {
		return nil
	}
	q := tsAllQueries.get(lang, queryStr)
	if q == nil {
		return nil
	}
	cursor := sitter.NewQueryCursor()
	if cursor == nil {
		return nil
	}
	defer cursor.Close()
	cursor.Exec(q, root)

	type quad struct {
		name      string
		typeStr   string
		lineNum   int
	}
	var results []globVar
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}
		var isTopLevel bool
		var pairs []quad
		for _, c := range m.Captures {
			if c.Node == nil {
				continue
			}
			switch q.CaptureNameForId(c.Index) {
			case "decl":
				parent := c.Node.Parent()
				isTopLevel = parent != nil && parent.Type() == topLevelType
			case "name":
				pairs = append(pairs, quad{
					name:    strings.TrimSpace(c.Node.Content(content)),
					lineNum: int(c.Node.StartPoint().Row) + 1,
				})
			case "type":
				if len(pairs) > 0 {
					pairs[len(pairs)-1].typeStr = strings.TrimSpace(c.Node.Content(content))
				}
			}
		}
		if isTopLevel {
			for _, p := range pairs {
				if p.name != "" {
					results = append(results, globVar{name: p.name, typeStr: p.typeStr, lineNum: p.lineNum})
				}
			}
		}
	}
	return results
}

// globVar 内部辅助结构，表示一个全局变量条目
type globVar struct {
	name    string
	typeStr string
	lineNum int
}

func (p *TreeSitterParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	def := p.def
	lang := def.GetLang()
	root, err := tsParseRootFor(content, lang)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filePath, err)
	}

	// 确定当前语言的顶层节点类型名
	topLevelType := topLevelNodeName(def.Name)
	pkgName := tsFirstFor(root, def.PkgQuery, "pkg", content, lang)
	var results []*model.GlobalVariable

	// Var 全局变量
	vars := tsGlobalsFor(root, def.VarQuery, content, lang, topLevelType)
	for _, v := range vars {
		results = append(results, &model.GlobalVariable{
			Name: v.name, VarType: v.typeStr, Language: def.Name,
			PackageName: pkgName, Visibility: visibilityFromName(v.name),
			FilePath: filepath.ToSlash(filePath), LineNum: v.lineNum, IsConst: false,
		})
	}

	// Const 常量（仅在查询不同时执行，避免与 Var 重复）
	if def.ConstQuery != "" && def.ConstQuery != def.VarQuery {
		consts := tsGlobalsFor(root, def.ConstQuery, content, lang, topLevelType)
		for _, c := range consts {
			results = append(results, &model.GlobalVariable{
				Name: c.name, VarType: c.typeStr, Language: def.Name,
				PackageName: pkgName, Visibility: visibilityFromName(c.name),
				FilePath: filepath.ToSlash(filePath), LineNum: c.lineNum, IsConst: true,
			})
		}
	}

	return results, nil
}

// topLevelNodeName 返回各语言语法树的顶层节点类型名
func topLevelNodeName(lang string) string {
	switch lang {
	case "python":
		return "module"
	case "java":
		return "program"
	case "javascript", "typescript":
		return "program"
	case "cpp":
		return "translation_unit"
	case "rust", "swift", "kotlin", "scala":
		return "source_file"
	case "ruby":
		return "program"
	case "csharp":
		return "compilation_unit"
	case "php":
		return "program"
	case "lua":
		return "program"
	default:
		return "source_file"
	}
}

// visibilityFromName 根据首字母判断可见性（多语言通用）
func visibilityFromName(name string) string {
	if len(name) == 0 {
		return "private"
	}
	if name[0] >= 'A' && name[0] <= 'Z' {
		return "public"
	}
	return "private"
}

// tsAllQueryCache 缓存 tree-sitter Query 对象，避免每次解析重复编译
// key = fmt.Sprintf("%p|%s", lang, queryStr)
// lang 是包级单例，跨 goroutine 指针稳定
type tsAllQueryCache struct {
	mu   sync.Mutex
	m    map[string]*sitter.Query
}

func (c *tsAllQueryCache) get(lang *sitter.Language, queryStr string) *sitter.Query {
	key := fmt.Sprintf("%p|%s", lang, queryStr)
	c.mu.Lock()
	if q, ok := c.m[key]; ok {
		c.mu.Unlock()
		return q
	}
	c.mu.Unlock()

	q, err := sitter.NewQuery([]byte(queryStr), lang)
	if err != nil || q == nil {
		return nil
	}

	c.mu.Lock()
	c.m[key] = q
	c.mu.Unlock()
	return q
}

var tsAllQueries = &tsAllQueryCache{m: make(map[string]*sitter.Query)}

// allParserPool 复用多语言 tree-sitter Parser 对象
// 注意：每个 Parser 取出后需调用 SetLanguage 设置当前语言
var allParserPool = sync.Pool{
	New: func() interface{} {
		p := sitter.NewParser()
		if p == nil {
			return nil
		}
		return p
	},
}

// ─── 通用树操作 ──────────────────────────────────────

func tsParseRootFor(content []byte, lang *sitter.Language) (*sitter.Node, error) {
	p := allParserPool.Get().(*sitter.Parser)
	if p == nil {
		return nil, fmt.Errorf("NewParser failed")
	}
	defer allParserPool.Put(p)
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
	q := tsAllQueries.get(lang, queryStr)
	if q == nil {
		return
	}
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
	q := tsAllQueries.get(lang, queryStr)
	if q == nil {
		return ""
	}
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
	q := tsAllQueries.get(lang, queryStr)
	if q == nil {
		return 0, 0
	}
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
	q := tsAllQueries.get(lang, queryStr)
	if q == nil {
		return nil
	}
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
	q := tsAllQueries.get(lang, callQuery)
	if q != nil {
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
		q2 := tsAllQueries.get(lang, selCallQuery)
		if q2 != nil {
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

	// 嵌套深度（内联计算）
	stats.NestingDepth = tsNestingDepth(bodyNode)
	return stats
}

// tsNestingDepth 递归计算函数体内的调用嵌套最大深度
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
