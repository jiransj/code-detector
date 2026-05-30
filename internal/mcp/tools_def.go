package mcp

import (
	"context"
	"encoding/json"

	"code-detector/internal/db"

	"github.com/mark3labs/mcp-go/mcp"
)

// ToolHandler 封装一个 MCP Tool 的元数据和 handler
type ToolHandler struct {
	Tool    mcp.Tool
	Handler func(ctx context.Context, req mcp.CallToolRequest, store *db.Store) (*mcp.CallToolResult, error)
}

// defineTools 定义所有 MCP Tool
func defineTools() []ToolHandler {
	return []ToolHandler{
		{
			Tool: mcp.NewTool("get_summary",
				mcp.WithDescription("获取数据库概要，包含扫描会话数、函数总数、全局变量数、依赖关系数、体总行数、最近扫描信息及语言分布统计"),
			),
			Handler: handleSummary,
		},
		{
			Tool: mcp.NewTool("list_functions",
				mcp.WithDescription("列出所有函数或按语言筛选。返回每个函数的名称、文件路径、行号范围、调用次数和嵌套深度"),
				mcp.WithString("language",
					mcp.Description("按语言筛选，如 go/python/java/javascript/typescript/cpp/csharp/rust/kotlin/ruby/swift/php/lua/scala，为空则返回全部"),
				),
			),
			Handler: handleFunctions,
		},
		{
			Tool: mcp.NewTool("get_function",
				mcp.WithDescription("查看指定名称函数的详细信息，包括签名、参数、返回类型、圈复杂度、调用次数等"),
				mcp.WithString("name",
					mcp.Description("函数名称（支持模糊匹配）"),
					mcp.Required(),
				),
			),
			Handler: handleFuncDetail,
		},
		{
			Tool: mcp.NewTool("get_function_body",
				mcp.WithDescription("获取指定 ID 函数的函数体源代码"),
				mcp.WithString("func_id",
					mcp.Description("函数的数据库 ID"),
					mcp.Required(),
				),
			),
			Handler: handleFuncBody,
		},
		{
			Tool: mcp.NewTool("list_variables",
				mcp.WithDescription("列出全局变量，可按语言筛选。返回变量名称、类型、可见性、所属包等"),
				mcp.WithString("language",
					mcp.Description("按语言筛选，为空则返回全部"),
				),
			),
			Handler: handleVars,
		},
		{
			Tool: mcp.NewTool("analyze_deps",
				mcp.WithDescription("显示函数调用关系统计：每个函数被调用的次数、调用了多少不同函数、总调用次数"),
			),
			Handler: handleDeps,
		},
		{
			Tool: mcp.NewTool("find_callers",
				mcp.WithDescription("查看哪些函数调用了指定名称的函数"),
				mcp.WithString("func_name",
					mcp.Description("目标函数名称"),
					mcp.Required(),
				),
			),
			Handler: handleCallers,
		},
		{
			Tool: mcp.NewTool("find_dead_code",
				mcp.WithDescription("列出未被其他任何函数调用的死代码函数"),
			),
			Handler: handleDead,
		},
		{
			Tool: mcp.NewTool("find_missing_deps",
				mcp.WithDescription("列出被调用但在数据库中找不到定义的缺失依赖函数"),
			),
			Handler: handleMissing,
		},
		{
			Tool: mcp.NewTool("top_functions",
				mcp.WithDescription("按函数体行数降序排列，列出最大的 N 个函数"),
				mcp.WithString("n",
					mcp.Description("返回前 N 个，默认 10"),
				),
			),
			Handler: handleTop,
		},
		{
			Tool: mcp.NewTool("deep_nesting",
				mcp.WithDescription("列出嵌套深度大于等于指定阈值的函数"),
				mcp.WithString("threshold",
					mcp.Description("嵌套深度阈值，默认 3"),
				),
			),
			Handler: handleDeep,
		},
		{
			Tool: mcp.NewTool("high_complexity",
				mcp.WithDescription("按圈复杂度降序排列，找出圈复杂度最高的 N 个函数。圈复杂度越高表示条件分支越多、代码越复杂"),
				mcp.WithString("limit",
					mcp.Description("返回前 N 个，默认 10"),
				),
			),
			Handler: handleComplexity,
		},
		{
			Tool: mcp.NewTool("many_params",
				mcp.WithDescription("列出参数数量大于等于指定阈值的函数"),
				mcp.WithString("threshold",
					mcp.Description("参数数量阈值，默认 5"),
				),
			),
			Handler: handleParams,
		},
		{
			Tool: mcp.NewTool("find_anonymous",
				mcp.WithDescription("列出包含匿名函数（闭包/内部函数）的函数"),
			),
			Handler: handleAnon,
		},
		{
			Tool: mcp.NewTool("file_metrics",
				mcp.WithDescription("获取文件级统计信息，包括每个文件的行数、函数数、类型数、复杂度指标等"),
			),
			Handler: handleFileMetrics,
		},
		{
			Tool: mcp.NewTool("list_types",
				mcp.WithDescription("列出所有类型定义（struct/interface/alias/enum 等），可按 kind 筛选"),
				mcp.WithString("kind",
					mcp.Description("类型种类筛选：struct / interface / alias / enum，为空则返回全部"),
				),
			),
			Handler: handleTypes,
		},
	}
}

// ─── Tool Handlers ──────────────────────────────────────

func handleSummary(ctx context.Context, req mcp.CallToolRequest, store *db.Store) (*mcp.CallToolResult, error) {
	summary, err := store.QuerySummary()
	if err != nil {
		return mcp.NewToolResultError("查询摘要失败: " + err.Error()), nil
	}
	return mcp.NewToolResultJSON(summary)
}

func handleFunctions(ctx context.Context, req mcp.CallToolRequest, store *db.Store) (*mcp.CallToolResult, error) {
	lang := req.GetString("language", "")
	if lang != "" {
		funcs, err := store.QueryFunctionsByLanguage(lang)
		if err != nil {
			return mcp.NewToolResultError("按语言查询函数失败: " + err.Error()), nil
		}
		return mcp.NewToolResultJSON(funcs)
	}
	funcs, err := store.QueryAllFunctions()
	if err != nil {
		return mcp.NewToolResultError("查询函数列表失败: " + err.Error()), nil
	}
	return mcp.NewToolResultJSON(funcs)
}

func handleFuncDetail(ctx context.Context, req mcp.CallToolRequest, store *db.Store) (*mcp.CallToolResult, error) {
	name := req.GetString("name", "")
	funcs, err := store.QueryFuncDetail(name)
	if err != nil {
		return mcp.NewToolResultError("查询函数详情失败: " + err.Error()), nil
	}
	return mcp.NewToolResultJSON(funcs)
}

func handleFuncBody(ctx context.Context, req mcp.CallToolRequest, store *db.Store) (*mcp.CallToolResult, error) {
	funcID := int64(req.GetInt("func_id", 0))
	if funcID <= 0 {
		return mcp.NewToolResultError("func_id 参数无效，请提供正整数 ID"), nil
	}
	body, err := store.QueryFuncBody(funcID)
	if err != nil {
		return mcp.NewToolResultError("查询函数体失败: " + err.Error()), nil
	}
	return mcp.NewToolResultText(body), nil
}

func handleVars(ctx context.Context, req mcp.CallToolRequest, store *db.Store) (*mcp.CallToolResult, error) {
	lang := req.GetString("language", "")
	if lang != "" {
		vars, err := store.QueryGlobalVarsByLanguage(lang)
		if err != nil {
			return mcp.NewToolResultError("按语言查询变量失败: " + err.Error()), nil
		}
		return mcp.NewToolResultJSON(vars)
	}
	vars, err := store.QueryVars()
	if err != nil {
		return mcp.NewToolResultError("查询变量失败: " + err.Error()), nil
	}
	return mcp.NewToolResultJSON(vars)
}

func handleDeps(ctx context.Context, req mcp.CallToolRequest, store *db.Store) (*mcp.CallToolResult, error) {
	stats, err := store.QueryDepStats()
	if err != nil {
		return mcp.NewToolResultError("查询调用统计失败: " + err.Error()), nil
	}
	return mcp.NewToolResultJSON(stats)
}

func handleCallers(ctx context.Context, req mcp.CallToolRequest, store *db.Store) (*mcp.CallToolResult, error) {
	funcName := req.GetString("func_name", "")
	callers, err := store.QueryCallers(funcName)
	if err != nil {
		return mcp.NewToolResultError("查询调用方失败: " + err.Error()), nil
	}
	return mcp.NewToolResultJSON(callers)
}

func handleDead(ctx context.Context, req mcp.CallToolRequest, store *db.Store) (*mcp.CallToolResult, error) {
	funcs, err := store.QueryDead()
	if err != nil {
		return mcp.NewToolResultError("查询死代码失败: " + err.Error()), nil
	}
	return mcp.NewToolResultJSON(funcs)
}

func handleMissing(ctx context.Context, req mcp.CallToolRequest, store *db.Store) (*mcp.CallToolResult, error) {
	missing, err := store.QueryMissing()
	if err != nil {
		return mcp.NewToolResultError("查询缺失依赖失败: " + err.Error()), nil
	}
	return mcp.NewToolResultJSON(missing)
}

func handleTop(ctx context.Context, req mcp.CallToolRequest, store *db.Store) (*mcp.CallToolResult, error) {
	n := req.GetInt("n", 10)
	if n < 1 {
		n = 10
	}
	funcs, err := store.QueryTop(n)
	if err != nil {
		return mcp.NewToolResultError("查询最大函数失败: " + err.Error()), nil
	}
	return mcp.NewToolResultJSON(funcs)
}

func handleDeep(ctx context.Context, req mcp.CallToolRequest, store *db.Store) (*mcp.CallToolResult, error) {
	threshold := req.GetInt("threshold", 3)
	if threshold < 1 {
		threshold = 3
	}
	funcs, err := store.QueryDeepNesting(threshold)
	if err != nil {
		return mcp.NewToolResultError("查询深度嵌套函数失败: " + err.Error()), nil
	}
	return mcp.NewToolResultJSON(funcs)
}

func handleComplexity(ctx context.Context, req mcp.CallToolRequest, store *db.Store) (*mcp.CallToolResult, error) {
	limit := req.GetInt("limit", 10)
	if limit < 1 {
		limit = 10
	}
	funcs, err := store.QueryByComplexity(limit)
	if err != nil {
		return mcp.NewToolResultError("查询复杂度失败: " + err.Error()), nil
	}
	return mcp.NewToolResultJSON(funcs)
}

func handleParams(ctx context.Context, req mcp.CallToolRequest, store *db.Store) (*mcp.CallToolResult, error) {
	threshold := req.GetInt("threshold", 5)
	if threshold < 1 {
		threshold = 5
	}
	funcs, err := store.QueryByParams(threshold)
	if err != nil {
		return mcp.NewToolResultError("查询参数失败: " + err.Error()), nil
	}
	return mcp.NewToolResultJSON(funcs)
}

func handleAnon(ctx context.Context, req mcp.CallToolRequest, store *db.Store) (*mcp.CallToolResult, error) {
	funcs, err := store.QueryAnonFuncs()
	if err != nil {
		return mcp.NewToolResultError("查询匿名函数失败: " + err.Error()), nil
	}
	return mcp.NewToolResultJSON(funcs)
}

func handleFileMetrics(ctx context.Context, req mcp.CallToolRequest, store *db.Store) (*mcp.CallToolResult, error) {
	metrics, err := store.QueryFileMetrics()
	if err != nil {
		return mcp.NewToolResultError("查询文件统计失败: " + err.Error()), nil
	}
	return mcp.NewToolResultJSON(metrics)
}

func handleTypes(ctx context.Context, req mcp.CallToolRequest, store *db.Store) (*mcp.CallToolResult, error) {
	defs, err := store.QueryTypeDefs()
	if err != nil {
		return mcp.NewToolResultError("查询类型定义失败: " + err.Error()), nil
	}
	kind := req.GetString("kind", "")
	if kind != "" {
		filtered := make([]interface{}, 0)
		data, _ := json.Marshal(defs)
		var all []map[string]interface{}
		json.Unmarshal(data, &all)
		for _, d := range all {
			if d["kind"] == kind {
				filtered = append(filtered, d)
			}
		}
		return mcp.NewToolResultJSON(filtered)
	}
	return mcp.NewToolResultJSON(defs)
}
