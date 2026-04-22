package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// goCheckSumTypeBinary returns the path to go-check-sumtype, installing it
// via `go install` if not already in PATH. Skips the test if unavailable.
func goCheckSumTypeBinary(t *testing.T) string {
	t.Helper()
	if path, err := exec.LookPath("go-check-sumtype"); err == nil {
		return path
	}
	install := exec.Command("go", "install", "github.com/alecthomas/go-check-sumtype/cmd/go-check-sumtype@latest")
	if out, err := install.CombinedOutput(); err != nil {
		t.Skipf("go-check-sumtype not in PATH and install failed: %s", out)
	}
	gopath, err := exec.Command("go", "env", "GOPATH").Output()
	if err != nil {
		t.Skip("cannot determine GOPATH")
	}
	binPath := filepath.Join(strings.TrimSpace(string(gopath)), "bin", "go-check-sumtype")
	if _, err := os.Stat(binPath); err != nil {
		t.Skipf("go-check-sumtype not found at %s after install", binPath)
	}
	return binPath
}

// setupCheckPackage writes a complete Go module to a temp dir, generates stubs
// via process(), then writes the provided switch file as use.go.
func setupCheckPackage(t *testing.T, typesSrc, switchSrc string) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("go.mod", "module p\ngo 1.21\n")
	write("types.go", typesSrc)
	if err := process(filepath.Join(dir, "types.go")); err != nil {
		t.Fatalf("process: %v", err)
	}
	write("use.go", switchSrc)
	return dir
}

const checkShapeTypesSrc = `package p

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
`

const exhaustiveShapeSwitchSrc = `package p

import "math"

func area(s Shape) float64 {
	switch v := s.(type) {
	case *Circle:
		return math.Pi * v.Radius * v.Radius
	case *Square:
		return v.Side * v.Side
	case *Triangle:
		return 0.5 * v.Base * v.Height
	}
	panic("unreachable")
}
`

const nonExhaustiveShapeSwitchSrc = `package p

func describe(s Shape) string {
	switch s.(type) {
	case *Circle:
		return "circle"
	case *Square:
		return "square"
	// missing *Triangle
	}
	panic("unreachable")
}
`

func TestGoCheckSumType_exhaustiveSwitchPasses(t *testing.T) {
	binary := goCheckSumTypeBinary(t)
	dir := setupCheckPackage(t, checkShapeTypesSrc, exhaustiveShapeSwitchSrc)

	cmd := exec.Command(binary, "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("expected clean output for exhaustive switch, got error:\n%s", out)
	}
}

func TestGoCheckSumType_nonExhaustiveSwitchFails(t *testing.T) {
	binary := goCheckSumTypeBinary(t)
	dir := setupCheckPackage(t, checkShapeTypesSrc, nonExhaustiveShapeSwitchSrc)

	cmd := exec.Command(binary, "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected go-check-sumtype to fail on non-exhaustive switch\noutput: %s", out)
	}
	if !strings.Contains(string(out), "Triangle") {
		t.Errorf("expected error to mention Triangle; got:\n%s", out)
	}
}

func TestGoCheckSumType_examplesAreExhaustive(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not in PATH")
	}
	binary := goCheckSumTypeBinary(t)

	cmd := exec.Command(binary, "./examples/...")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go-check-sumtype found issues in examples:\n%s", out)
	}
}

const checkMultiTypesSrc = `package p

//sumtype:decl
type Color interface {
	isColor()
}

//gosumtype:impl Color
type (
	Red   struct{}
	Blue  struct{}
	Green struct{}
)

//sumtype:decl
type Size interface {
	isSize()
}

//gosumtype:impl Size
type (
	Small  struct{}
	Medium struct{}
	Large  struct{}
)
`

const exhaustiveMultiSwitchSrc = `package p

func colorName(c Color) string {
	switch c.(type) {
	case *Red:
		return "red"
	case *Blue:
		return "blue"
	case *Green:
		return "green"
	}
	panic("unreachable")
}

func sizeName(s Size) string {
	switch s.(type) {
	case *Small:
		return "small"
	case *Medium:
		return "medium"
	case *Large:
		return "large"
	}
	panic("unreachable")
}
`

const nonExhaustiveMultiSwitchSrc = `package p

func colorName(c Color) string {
	switch c.(type) {
	case *Red:
		return "red"
	case *Blue:
		return "blue"
	// missing *Green
	}
	panic("unreachable")
}

func sizeName(s Size) string {
	switch s.(type) {
	case *Small:
		return "small"
	// missing *Medium, *Large
	}
	panic("unreachable")
}
`

func TestGoCheckSumType_exhaustiveMultipleSumTypesPasses(t *testing.T) {
	binary := goCheckSumTypeBinary(t)
	dir := setupCheckPackage(t, checkMultiTypesSrc, exhaustiveMultiSwitchSrc)

	cmd := exec.Command(binary, "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("expected clean output for exhaustive multi-sumtype switch, got:\n%s", out)
	}
}

func TestGoCheckSumType_nonExhaustiveMultipleSumTypesFails(t *testing.T) {
	binary := goCheckSumTypeBinary(t)
	dir := setupCheckPackage(t, checkMultiTypesSrc, nonExhaustiveMultiSwitchSrc)

	cmd := exec.Command(binary, "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected failure for non-exhaustive multi-sumtype switch\noutput: %s", out)
	}
	if !strings.Contains(string(out), "Green") {
		t.Errorf("expected error to mention Green; got:\n%s", out)
	}
}
