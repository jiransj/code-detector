package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"code-detector/internal/db"

	"github.com/mark3labs/mcp-go/mcp"
)

// ResourceHandler 封装一个 MCP Resource 的元数据和 handler
type ResourceHandler struct {
	Resource mcp.Resource
	Handler  func(ctx context.Context, req mcp.ReadResourceRequest, store *db.Store) ([]mcp.ResourceContents, error)
}

// defineResources 定义所有 MCP Resource
func defineResources() []ResourceHandler {
	return []ResourceHandler{
		{
			Resource: mcp.NewResource("db://summary", "数据库概要",
				mcp.WithResourceDescription("扫描数据库概要信息，包含会话数、函数数、变量数、语言分布等统计"),
				mcp.WithMIMEType("application/json"),
			),
			Handler: resourceSummary,
		},
		{
			Resource: mcp.NewResource("db://functions", "函数列表",
				mcp.WithResourceDescription("所有函数的完整列表（不含函数体）"),
				mcp.WithMIMEType("application/json"),
			),
			Handler: resourceFunctions,
		},
		{
			Resource: mcp.NewResource("db://variables", "全局变量",
				mcp.WithResourceDescription("所有全局变量的完整列表"),
				mcp.WithMIMEType("application/json"),
			),
			Handler: resourceVariables,
		},
		{
			Resource: mcp.NewResource("db://files", "文件统计",
				mcp.WithResourceDescription("所有文件的统计信息，包含行数、函数数、复杂度等指标"),
				mcp.WithMIMEType("application/json"),
			),
			Handler: resourceFiles,
		},
		{
			Resource: mcp.NewResource("db://types", "类型定义",
				mcp.WithResourceDescription("所有类型定义的列表（struct/interface/alias/enum）"),
				mcp.WithMIMEType("application/json"),
			),
			Handler: resourceTypes,
		},
		{
			Resource: mcp.NewResource("db://sessions/latest", "最近扫描会话",
				mcp.WithResourceDescription("最近一次扫描会话的详细信息"),
				mcp.WithMIMEType("application/json"),
			),
			Handler: resourceLatestSession,
		},
	}
}

// ─── Resource Handlers ──────────────────────────────────

func resourceSummary(ctx context.Context, req mcp.ReadResourceRequest, store *db.Store) ([]mcp.ResourceContents, error) {
	summary, err := store.QuerySummary()
	if err != nil {
		return nil, fmt.Errorf("查询摘要失败: %w", err)
	}
	data, _ := json.MarshalIndent(summary, "", "  ")
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "db://summary",
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func resourceFunctions(ctx context.Context, req mcp.ReadResourceRequest, store *db.Store) ([]mcp.ResourceContents, error) {
	funcs, err := store.QueryAllFunctions()
	if err != nil {
		return nil, fmt.Errorf("查询函数列表失败: %w", err)
	}
	data, _ := json.MarshalIndent(funcs, "", "  ")
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "db://functions",
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func resourceVariables(ctx context.Context, req mcp.ReadResourceRequest, store *db.Store) ([]mcp.ResourceContents, error) {
	vars, err := store.QueryVars()
	if err != nil {
		return nil, fmt.Errorf("查询变量失败: %w", err)
	}
	data, _ := json.MarshalIndent(vars, "", "  ")
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "db://variables",
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func resourceFiles(ctx context.Context, req mcp.ReadResourceRequest, store *db.Store) ([]mcp.ResourceContents, error) {
	metrics, err := store.QueryFileMetrics()
	if err != nil {
		return nil, fmt.Errorf("查询文件统计失败: %w", err)
	}
	data, _ := json.MarshalIndent(metrics, "", "  ")
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "db://files",
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func resourceTypes(ctx context.Context, req mcp.ReadResourceRequest, store *db.Store) ([]mcp.ResourceContents, error) {
	defs, err := store.QueryTypeDefs()
	if err != nil {
		return nil, fmt.Errorf("查询类型定义失败: %w", err)
	}
	data, _ := json.MarshalIndent(defs, "", "  ")
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "db://types",
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func resourceLatestSession(ctx context.Context, req mcp.ReadResourceRequest, store *db.Store) ([]mcp.ResourceContents, error) {
	summary, err := store.QuerySummary()
	if err != nil {
		return nil, fmt.Errorf("查询会话失败: %w", err)
	}
	if latest, ok := summary["latest_session"]; ok && latest != nil {
		data, _ := json.MarshalIndent(latest, "", "  ")
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "db://sessions/latest",
				MIMEType: "application/json",
				Text:     string(data),
			},
		}, nil
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "db://sessions/latest",
			MIMEType: "application/json",
			Text:     "{}",
		},
	}, nil
}
