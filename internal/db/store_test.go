package db

import (
	"testing"

	"code-detector/internal/model"
)

func TestFuncHashStable(t *testing.T) {
	f := &model.Function{
		Name:        "DoSomething",
		PackageName: "mypkg",
		Language:    "go",
		FilePath:    "main.go",
		LineStart:   10,
		LineEnd:     30,
		Body:        "func DoSomething() { return }",
		Dependencies: []string{"helper", "printer"},
		CallCount:    3,
		NestingDepth: 1,
	}

	h1 := FuncHash(f)
	h2 := FuncHash(f)
	if h1 != h2 {
		t.Errorf("FuncHash not stable: h1=%s h2=%s", h1, h2)
	}
}

func TestFuncHashDiffersOnChange(t *testing.T) {
	f1 := &model.Function{
		Name:     "Foo",
		Language: "go",
		FilePath: "a.go",
		LineStart: 1, LineEnd: 5,
		Body: "func Foo() {}",
	}
	f2 := &model.Function{
		Name:     "Foo",
		Language: "go",
		FilePath: "a.go",
		LineStart: 1, LineEnd: 10,
		Body: "func Foo() { return 1 }",
	}

	h1 := FuncHash(f1)
	h2 := FuncHash(f2)
	if h1 == h2 {
		t.Errorf("expected different hashes for different functions")
	}
}

func TestVarHashStable(t *testing.T) {
	v := &model.GlobalVariable{
		Name:       "MAX_SIZE",
		VarType:    "int",
		Language:   "go",
		PackageName: "main",
		Visibility: "public",
		FilePath:   "main.go",
		LineNum:    5,
		IsConst:    true,
	}

	h1 := VarHash(v)
	h2 := VarHash(v)
	if h1 != h2 {
		t.Errorf("VarHash not stable: h1=%s h2=%s", h1, h2)
	}
}

func TestBuildInClause(t *testing.T) {
	// Test with values
	clause, args := buildInClause("name", []string{"a", "b", "c"})
	if clause != "name IN (?, ?, ?)" {
		t.Errorf("clause = %q, want \"name IN (?, ?, ?)\"", clause)
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}

	// Test empty
	emptyClause, emptyArgs := buildInClause("name", nil)
	if emptyClause != "name IN (NULL)" {
		t.Errorf("empty clause = %q, want \"name IN (NULL)\"", emptyClause)
	}
	if emptyArgs != nil {
		t.Errorf("expected nil args for empty list")
	}
}
