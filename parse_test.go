package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func parseSource(t *testing.T, src string) ([]sumType, error) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("test source did not parse: %v", err)
	}
	return parseSumTypes(f)
}

func mustParseSource(t *testing.T, src string) []sumType {
	t.Helper()
	result, err := parseSource(t, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return result
}

func tempGoFile(t *testing.T, src string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "input.go")
	if err := os.WriteFile(f, []byte(src), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return f
}

// --- happy path ---

func TestParse_basicSumType(t *testing.T) {
	t.Parallel()
	result := mustParseSource(t, `package p

//sumtype:decl
type Shape interface {
	isShape()
}

//gosumtype:impl Shape
type (
	Circle   struct{ Radius float64 }
	Square   struct{ Side float64 }
	Triangle struct{ Base, Height float64 }
)
`)
	if len(result) != 1 {
		t.Fatalf("expected 1 sumtype, got %d", len(result))
	}
	st := result[0]
	if st.pkg != "p" {
		t.Errorf("pkg: got %q, want %q", st.pkg, "p")
	}
	if st.ifaceName != "Shape" {
		t.Errorf("ifaceName: got %q, want %q", st.ifaceName, "Shape")
	}
	if st.markerMethod != "isShape" {
		t.Errorf("markerMethod: got %q, want %q", st.markerMethod, "isShape")
	}
	if len(st.variants) != 3 {
		t.Fatalf("variants: got %d, want 3", len(st.variants))
	}
	if st.variants[0].name != "Circle" || st.variants[1].name != "Square" || st.variants[2].name != "Triangle" {
		t.Errorf("variant names: got %v", []string{st.variants[0].name, st.variants[1].name, st.variants[2].name})
	}
}

func TestParse_singleTypeParam(t *testing.T) {
	t.Parallel()
	result := mustParseSource(t, `package p

//sumtype:decl
type Wrapper interface { isWrapper() }

//gosumtype:impl Wrapper
type (
	Box[T any] struct{ Value T }
)
`)
	if len(result) != 1 {
		t.Fatalf("expected 1 sumtype, got %d", len(result))
	}
	v := result[0].variants[0]
	if v.name != "Box" {
		t.Errorf("name: got %q, want Box", v.name)
	}
	if len(v.typeParams) != 1 || v.typeParams[0] != "T" {
		t.Errorf("typeParams: got %v, want [T]", v.typeParams)
	}
}

func TestParse_multipleTypeParams(t *testing.T) {
	t.Parallel()
	result := mustParseSource(t, `package p

//sumtype:decl
type Pair interface { isPair() }

//gosumtype:impl Pair
type (
	KVPair[K comparable, V any] struct {
		Key   K
		Value V
	}
)
`)
	if len(result) != 1 {
		t.Fatalf("expected 1 sumtype, got %d", len(result))
	}
	v := result[0].variants[0]
	if len(v.typeParams) != 2 || v.typeParams[0] != "K" || v.typeParams[1] != "V" {
		t.Errorf("typeParams: got %v, want [K V]", v.typeParams)
	}
}

func TestParse_multipleTypeParamsOnSameField(t *testing.T) {
	t.Parallel()
	result := mustParseSource(t, `package p

//sumtype:decl
type Node interface { isNode() }

//gosumtype:impl Node
type (
	Edge[A, B comparable] struct{}
)
`)
	if len(result) != 1 {
		t.Fatalf("expected 1 sumtype, got %d", len(result))
	}
	v := result[0].variants[0]
	if len(v.typeParams) != 2 || v.typeParams[0] != "A" || v.typeParams[1] != "B" {
		t.Errorf("typeParams: got %v, want [A B]", v.typeParams)
	}
}

func TestParse_multipleSumTypesInOneFile(t *testing.T) {
	t.Parallel()
	result := mustParseSource(t, `package p

//sumtype:decl
type Shape interface{ isShape() }

//gosumtype:impl Shape
type (
	Circle struct{}
	Square struct{}
)

//sumtype:decl
type Color interface{ isColor() }

//gosumtype:impl Color
type (
	Red  struct{}
	Blue struct{}
)
`)
	if len(result) != 2 {
		t.Fatalf("expected 2 sumtypes, got %d", len(result))
	}
	if result[0].ifaceName != "Shape" {
		t.Errorf("first: got %q, want Shape", result[0].ifaceName)
	}
	if result[1].ifaceName != "Color" {
		t.Errorf("second: got %q, want Color", result[1].ifaceName)
	}
}

func TestParse_variantsBlockWithoutAnnotationIsIgnored(t *testing.T) {
	t.Parallel()
	result := mustParseSource(t, `package p

//sumtype:decl
type Shape interface{ isShape() }

type (
	Circle struct{}
	Square struct{}
)
`)
	if len(result) != 0 {
		t.Errorf("expected 0 sumtypes (unannotated block ignored), got %d", len(result))
	}
}

func TestParse_interfaceMethodNameNotDerivedFromName(t *testing.T) {
	t.Parallel()
	// Method name doesn't have to follow is+InterfaceName — generator reads whatever is in the interface.
	result := mustParseSource(t, `package p

//sumtype:decl
type Shape interface {
	isShapeVariant()
}

//gosumtype:impl Shape
type (
	Circle struct{}
	Square struct{}
)
`)
	if len(result) != 1 {
		t.Fatalf("expected 1 sumtype, got %d", len(result))
	}
	if result[0].markerMethod != "isShapeVariant" {
		t.Errorf("markerMethod: got %q, want isShapeVariant", result[0].markerMethod)
	}
}

func TestParse_interfaceWithMultipleMethodsPicksMarker(t *testing.T) {
	t.Parallel()
	result := mustParseSource(t, `package p

//sumtype:decl
type Animal interface {
	isAnimal()
	String() string
}

//gosumtype:impl Animal
type (
	Dog struct{}
	Cat struct{}
)
`)
	if len(result) != 1 {
		t.Fatalf("expected 1 sumtype, got %d", len(result))
	}
	if result[0].markerMethod != "isAnimal" {
		t.Errorf("markerMethod: got %q, want isAnimal", result[0].markerMethod)
	}
}

func TestParse_variantsPreserveOrder(t *testing.T) {
	t.Parallel()
	result := mustParseSource(t, `package p

//sumtype:decl
type Expr interface{ isExpr() }

//gosumtype:impl Expr
type (
	LitExpr  struct{}
	AddExpr  struct{}
	MulExpr  struct{}
	CallExpr struct{}
)
`)
	if len(result) != 1 {
		t.Fatalf("expected 1 sumtype, got %d", len(result))
	}
	want := []string{"LitExpr", "AddExpr", "MulExpr", "CallExpr"}
	for i, w := range want {
		if result[0].variants[i].name != w {
			t.Errorf("variants[%d]: got %q, want %q", i, result[0].variants[i].name, w)
		}
	}
}

func TestParse_emptyVariantsBlockProducesNoResult(t *testing.T) {
	t.Parallel()
	// Empty type block: go parser won't even parse "type ()" but a block with
	// only non-TypeSpec entries is handled gracefully.
	result := mustParseSource(t, `package p

//sumtype:decl
type Shape interface{ isShape() }
`)
	// No //gosumtype:impl  block at all → 0 results.
	if len(result) != 0 {
		t.Errorf("expected 0 sumtypes, got %d", len(result))
	}
}

func TestParse_interfaceDefinedAfterVariantsBlock(t *testing.T) {
	t.Parallel()
	// Interface can appear anywhere in the file; buildIfaceMap scans the whole file.
	result := mustParseSource(t, `package p

//gosumtype:impl Shape
type (
	Circle struct{}
	Square struct{}
)

//sumtype:decl
type Shape interface{ isShape() }
`)
	if len(result) != 1 {
		t.Fatalf("expected 1 sumtype, got %d", len(result))
	}
	if result[0].ifaceName != "Shape" {
		t.Errorf("ifaceName: got %q, want Shape", result[0].ifaceName)
	}
}

func TestParse_goSumTypeAnnotationParsing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text   string
		name   string
		wantOK bool
	}{
		{"//gosumtype:impl Shape", "Shape", true}, // directive form — gofmt preserves
		{"//gosumtype:impl Expr", "Expr", true},
		{"//gosumtype:impl ", "", false},           // empty name
		{"//sumtype:decl", "", false},              // linter annotation, not generator
		{"// gosumtype:Shape", "", false},          // space after // → not matched
		{"//gosumtype:impl  Shape", "Shape", true}, // trailing space in name trimmed
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()
			got, ok := parseGoSumTypeAnnotation(tt.text)
			if ok != tt.wantOK {
				t.Errorf("ok: got %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.name {
				t.Errorf("name: got %q, want %q", got, tt.name)
			}
		})
	}
}

// --- error cases ---

func TestParse_unknownInterface(t *testing.T) {
	t.Parallel()
	_, err := parseSource(t, `package p

//gosumtype:impl DoesNotExist
type (
	Foo struct{}
)
`)
	if err == nil {
		t.Fatal("expected error for unknown interface, got nil")
	}
	if !strings.Contains(err.Error(), "DoesNotExist") {
		t.Errorf("error should mention interface name: %v", err)
	}
}

func TestParse_interfaceWithNoMethods(t *testing.T) {
	t.Parallel()
	_, err := parseSource(t, `package p

//sumtype:decl
type Empty interface{}

//gosumtype:impl Empty
type (
	Foo struct{}
)
`)
	if err == nil {
		t.Fatal("expected error for interface with no methods, got nil")
	}
	if !strings.Contains(err.Error(), "marker method") {
		t.Errorf("error should mention marker method: %v", err)
	}
}

func TestParse_interfaceWithOnlyExportedMethods(t *testing.T) {
	t.Parallel()
	_, err := parseSource(t, `package p

//sumtype:decl
type Animal interface {
	String() string
}

//gosumtype:impl Animal
type (
	Dog struct{}
)
`)
	if err == nil {
		t.Fatal("expected error for interface with only exported methods, got nil")
	}
	if !strings.Contains(err.Error(), "marker method") {
		t.Errorf("error should mention marker method: %v", err)
	}
}

func TestParse_interfaceWithOnlyEmbeddedInterface(t *testing.T) {
	t.Parallel()
	_, err := parseSource(t, `package p

//sumtype:decl
type Animal interface {
	fmt.Stringer
}

//gosumtype:impl Animal
type (
	Dog struct{}
)
`)
	if err == nil {
		t.Fatal("expected error for interface with only embedded type, got nil")
	}
	if !strings.Contains(err.Error(), "marker method") {
		t.Errorf("error should mention marker method: %v", err)
	}
}

func TestParse_markerMethodWithParams(t *testing.T) {
	t.Parallel()
	_, err := parseSource(t, `package p

//sumtype:decl
type Thing interface {
	isThing(x int)
}

//gosumtype:impl Thing
type (
	Foo struct{}
)
`)
	if err == nil {
		t.Fatal("expected error for marker method with params, got nil")
	}
	if !strings.Contains(err.Error(), "marker method") {
		t.Errorf("error should mention marker method: %v", err)
	}
}

func TestParse_markerMethodWithReturn(t *testing.T) {
	t.Parallel()
	_, err := parseSource(t, `package p

//sumtype:decl
type Thing interface {
	isThing() bool
}

//gosumtype:impl Thing
type (
	Foo struct{}
)
`)
	if err == nil {
		t.Fatal("expected error for marker method with return, got nil")
	}
	if !strings.Contains(err.Error(), "marker method") {
		t.Errorf("error should mention marker method: %v", err)
	}
}

// --- parseFile integration ---

func TestParseFile_readsFromDisk(t *testing.T) {
	t.Parallel()
	f := tempGoFile(t, `package p

//sumtype:decl
type Shape interface{ isShape() }

//gosumtype:impl Shape
type (
	Circle struct{}
)
`)
	result, err := parseFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 sumtype, got %d", len(result))
	}
}

func TestParseFile_syntaxError(t *testing.T) {
	t.Parallel()
	f := tempGoFile(t, `package p
this is not {{{ valid go`)
	_, err := parseFile(f)
	if err == nil {
		t.Fatal("expected error for invalid Go syntax, got nil")
	}
}

func TestParseFile_nonexistentFile(t *testing.T) {
	t.Parallel()
	_, err := parseFile("/nonexistent/does_not_exist.go")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestParseFile_emptyFile(t *testing.T) {
	t.Parallel()
	f := tempGoFile(t, `package p`)
	result, err := parseFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 sumtypes, got %d", len(result))
	}
}

func TestParseFile_noAnnotations(t *testing.T) {
	t.Parallel()
	f := tempGoFile(t, `package p

type (
	Shape  interface{ String() string }
	Circle struct{}
)
`)
	result, err := parseFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 sumtypes, got %d", len(result))
	}
}
