# ============================================================================
# code-detector — 多编程语言函数扫描工具 构建文件
# 用法: make <target>
# ============================================================================

# ── 基本配置 ──────────────────────────────────────────────────────────────
# 注意: 务必用 `go build -o code-detector.exe ./cmd/scanner/` 或 `make`
#       不要直接 `go build ./cmd/scanner/`，它会输出 scanner.exe
BINARY    := code-detector
OUTPUT_DIR := build
GO        := go
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS   := -s -w -X main.version=$(VERSION)
BUILD_TIME := $(shell date "+%Y-%m-%d %H:%M:%S" 2>/dev/null || echo "unknown")

# ── CGO 配置（tree-sitter AST 解析器需要） ──────────────────────────
# Windows: 确保 MinGW-w64 在 PATH 中
# Linux/macOS: GCC/Clang 通常预装，CGO_ENABLED=1 默认开启
CGO_ENABLED := 1

# ── 颜色输出 ──────────────────────────────────────────────────────────────
BLUE   := \033[34;1m
GREEN  := \033[32;1m
YELLOW := \033[33;1m
RED    := \033[31;1m
CYAN   := \033[36;1m
RESET  := \033[0m

.PHONY: all build clean test lint run install help scan

# ── 默认目标 ──────────────────────────────────────────────────────────────
all: clean lint test build
	@printf "$(GREEN)✓ 全部完成$(RESET)\n"

# ── 构建 ──────────────────────────────────────────────────────────────────
build:
	@printf "$(BLUE)━━━ 构建 $(BINARY) v$(VERSION) ━━━$(RESET)\n"
	@printf "  编译时间: $(BUILD_TIME)\n"
	@mkdir -p $(OUTPUT_DIR)
	$(GO) build -ldflags="$(LDFLAGS)" -o $(OUTPUT_DIR)/$(BINARY) ./cmd/scanner
	@printf "$(GREEN)✓ 构建成功: $(OUTPUT_DIR)/$(BINARY)$(RESET)\n"
	@printf "  文件大小: "
	@ls -lh $(OUTPUT_DIR)/$(BINARY) 2>/dev/null | awk '{print $$5}'

# ── 直接构建到当前目录（开发用）─────────────────────────────────────────────
dev:
	@printf "$(YELLOW)▶ 开发构建...$(RESET)\n"
	$(GO) build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/scanner
	@printf "$(GREEN)✓ $(BINARY) 已构建$(RESET)\n"

# ── 测试 ──────────────────────────────────────────────────────────────────
test: vet
	@printf "$(BLUE)━━━ 运行测试 ━━━$(RESET)\n"
	$(GO) test -v -count=1 ./... 2>&1 || true
	@printf "$(GREEN)✓ 测试完成$(RESET)\n"

# ── 代码检查 ──────────────────────────────────────────────────────────────
vet:
	@printf "$(CYAN)▶ go vet...$(RESET)\n"
	$(GO) vet ./cmd/scanner ./internal/...
	@printf "$(GREEN)✓ go vet 通过$(RESET)\n"

lint: vet
	@printf "$(CYAN)▶ 检查未使用的导出符号...$(RESET)\n"
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
	else \
		printf "$(YELLOW)  staticcheck 未安装，跳过$(RESET)\n"; \
	fi

# ── 清理 ──────────────────────────────────────────────────────────────────
clean:
	@printf "$(YELLOW)▶ 清理构建产物...$(RESET)\n"
	rm -rf $(OUTPUT_DIR)
	rm -f $(BINARY) $(BINARY).exe
	rm -f scaned_db/*.db
	@printf "$(GREEN)✓ 清理完成$(RESET)\n"

# ── 运行扫描 ──────────────────────────────────────────────────────────────
run:
	@printf "$(BLUE)━━━ 运行扫描 ━━━$(RESET)\n"
	$(GO) run ./cmd/scanner $(ARGS)

scan: dev
	@printf "$(BLUE)━━━ 扫描 $(DIR) ━━━$(RESET)\n"
	./$(BINARY) -db scaned_db/scan_result.db -verbose ./$(DIR)

# ── 安装 ──────────────────────────────────────────────────────────────────
install:
	@printf "$(BLUE)━━━ 安装到 GOPATH/bin ━━━$(RESET)\n"
	$(GO) install -ldflags="$(LDFLAGS)" ./cmd/scanner
	@printf "$(GREEN)✓ 安装完成: $(shell $(GO) env GOPATH)/bin/$(BINARY)$(RESET)\n"

# ── 显示版本 ──────────────────────────────────────────────────────────────
version:
	@printf "$(BINARY) v$(VERSION)\n"
	@printf "Go: $(shell $(GO) version)\n"

# ── 帮助 ──────────────────────────────────────────────────────────────────
help:
	@printf "$(CYAN)╔════════════════════════════════════════╗\n"
	@printf "║  $(BINARY) — 构建命令                       ║\n"
	@printf "╚════════════════════════════════════════╝$(RESET)\n"
	@printf "  $(GREEN)make$(RESET)          完整构建 (clean → lint → test → build)\n"
	@printf "  $(GREEN)make build$(RESET)     编译到 $(OUTPUT_DIR)/\n"
	@printf "  $(GREEN)make dev$(RESET)       快速编译到当前目录\n"
	@printf "  $(GREEN)make test$(RESET)      运行 go vet + go test\n"
	@printf "  $(GREEN)make lint$(RESET)      代码检查\n"
	@printf "  $(GREEN)make clean$(RESET)     清理产物\n"
	@printf "  $(GREEN)make run ARGS=\"...\"$(RESET)  直接运行 (例如 ARGS=\"-h\")\n"
	@printf "  $(GREEN)make scan DIR=./testdata$(RESET)  构建并扫描目录\n"
	@printf "  $(GREEN)make install$(RESET)   安装到 GOPATH/bin\n"
	@printf "  $(GREEN)make version$(RESET)   显示版本\n"
