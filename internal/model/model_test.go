package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFunctionJSON(t *testing.T) {
	f := &Function{
		ID:           1,
		Name:         "TestFunc",
		PackageName:  "testpkg",
		Language:     "go",
		FilePath:     "main.go",
		LineStart:    10,
		LineEnd:      20,
		Body:         "func TestFunc() {}",
		Dependencies: []string{"helper"},
		CallCount:    2,
		NestingDepth: 1,
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var f2 Function
	if err := json.Unmarshal(data, &f2); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if f2.Name != f.Name {
		t.Errorf("Name: got %q, want %q", f2.Name, f.Name)
	}
	if f2.CallCount != f.CallCount {
		t.Errorf("CallCount: got %d, want %d", f2.CallCount, f.CallCount)
	}
	if len(f2.Dependencies) != 1 || f2.Dependencies[0] != "helper" {
		t.Errorf("Dependencies: got %v, want [helper]", f2.Dependencies)
	}
}

func TestScanResultAggregation(t *testing.T) {
	r := &ScanResult{
		Session: ScanSession{
			ProjectRoot: "/test",
			ScanTime:    time.Now(),
		},
		Functions: []*Function{
			{Name: "A"},
			{Name: "B"},
		},
		GlobalVars: []*GlobalVariable{
			{Name: "X"},
		},
		FileCount: 1,
		Duration:  100 * time.Millisecond,
	}

	if len(r.Functions) != 2 {
		t.Errorf("expected 2 functions, got %d", len(r.Functions))
	}
	if len(r.GlobalVars) != 1 {
		t.Errorf("expected 1 global var, got %d", len(r.GlobalVars))
	}
	if r.Session.ProjectRoot != "/test" {
		t.Errorf("expected /test, got %q", r.Session.ProjectRoot)
	}
}

func TestFunctionHashField(t *testing.T) {
	// 确保 Hash 字段在 JSON 中正确序列化
	f := &Function{
		Name: "Hashed",
		Hash: "abc123",
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var f2 Function
	if err := json.Unmarshal(data, &f2); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if f2.Hash != "abc123" {
		t.Errorf("Hash: got %q, want abc123", f2.Hash)
	}
}
