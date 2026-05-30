package mcp

import (
	"context"
	"fmt"
	"log"
	"os"

	"code-detector/internal/db"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RunStdioServer 以 stdio 模式启动 MCP Server
func RunStdioServer(store *db.Store) error {
	// 创建 MCP Server 实例
	s := server.NewMCPServer(
		"code-detector",
		"1.1",
		server.WithResourceCapabilities(true, true),
		server.WithToolCapabilities(true),
		server.WithLogging(),
	)

	// 注册所有工具
	registerTools(s, store)

	// 注册所有资源
	registerResources(s, store)

	// 创建 stdio server
	stdioServer := server.NewStdioServer(s)
	stdioServer.SetErrorLogger(log.New(os.Stderr, "[mcp] ", log.LstdFlags))

	fmt.Fprintf(os.Stderr, "[mcp] code-detector MCP Server 启动 (stdio 模式)\n")
	fmt.Fprintf(os.Stderr, "[mcp] 数据库: %s\n", store.DB)

	// 在 stdin/stdout 上监听 MCP 请求
	return stdioServer.Listen(context.Background(), os.Stdin, os.Stdout)
}

// registerTools 注册所有 MCP 工具及其 handler
func registerTools(s *server.MCPServer, store *db.Store) {
	tools := defineTools()
	for _, t := range tools {
		tool := t  // capture
		s.AddTool(tool.Tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return tool.Handler(ctx, req, store)
		})
	}
}

// registerResources 注册所有 MCP 资源
func registerResources(s *server.MCPServer, store *db.Store) {
	resources := defineResources()
	for _, r := range resources {
		res := r
		s.AddResource(res.Resource, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return res.Handler(ctx, req, store)
		})
	}
}
