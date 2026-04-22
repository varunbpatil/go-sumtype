//go:generate go-sumtype $GOFILE

package examples

import "math"

// Two independent sum types in one file demonstrating go-sumtype.
// Run `go generate` to regenerate shapes_sumtypes.go after modifying either block.

//sumtype:decl
type Shape interface {
	isShape()
}

//gosumtype:impl Shape
type (
	Circle struct {
		Radius float64
	}
	Square struct {
		Side float64
	}
	Triangle struct {
		Base   float64
		Height float64
	}
	// Scaled wraps any Shape and multiplies its area by Factor^2.
	Scaled[N interface{ ~float64 }] struct {
		Inner  Shape
		Factor N
	}
)

//sumtype:decl
type Expr interface {
	isExpr()
}

//gosumtype:impl Expr
type (
	LitExpr struct {
		Value int
	}
	AddExpr struct {
		Left  Expr
		Right Expr
	}
	NegExpr struct {
		Operand Expr
	}
)

// Area computes the area of a Shape. The switch is exhaustive — go-check-sumtype
// will report a lint error if a new Shape variant is added without updating this.
//
// Note: generic variants (Scaled[N]) must be handled with a concrete instantiation
// in type switches. go-check-sumtype does not enforce coverage of generic variants
// since all possible type arguments cannot be enumerated statically.
func Area(s Shape) float64 {
	switch v := s.(type) {
	case *Circle:
		return math.Pi * v.Radius * v.Radius
	case *Square:
		return v.Side * v.Side
	case *Triangle:
		return 0.5 * v.Base * v.Height
	case *Scaled[float64]:
		return Area(v.Inner) * v.Factor * v.Factor
	}
	panic("unreachable")
}

// Eval evaluates an Expr tree to an integer.
func Eval(e Expr) int {
	switch v := e.(type) {
	case *LitExpr:
		return v.Value
	case *AddExpr:
		return Eval(v.Left) + Eval(v.Right)
	case *NegExpr:
		return -Eval(v.Operand)
	}
	panic("unreachable")
}
