package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

type sumType struct {
	pkg          string
	ifaceName    string
	markerMethod string
	variants     []variant
}

type variant struct {
	name       string
	typeParams []string // param names only, no constraints
}

func parseFile(filename string) ([]sumType, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}
	return parseSumTypes(f)
}

func parseSumTypes(f *ast.File) ([]sumType, error) {
	ifaceMap := buildIfaceMap(f)
	var result []sumType
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		ifaceName, ok := variantBlockIface(gd)
		if !ok {
			continue
		}
		st, err := extractSumType(f.Name.Name, gd, ifaceName, ifaceMap)
		if err != nil {
			return nil, err
		}
		if st != nil {
			result = append(result, *st)
		}
	}
	return result, nil
}

// buildIfaceMap returns a map from interface name to its TypeSpec for all
// interfaces declared in the file.
func buildIfaceMap(f *ast.File) map[string]*ast.TypeSpec {
	m := make(map[string]*ast.TypeSpec)
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if _, isIface := ts.Type.(*ast.InterfaceType); isIface {
				m[ts.Name.Name] = ts
			}
		}
	}
	return m
}

// variantBlockIface returns the interface name from a //gosumtype:impl Name
// annotation on a GenDecl's doc comment.
func variantBlockIface(gd *ast.GenDecl) (string, bool) {
	if gd.Doc == nil {
		return "", false
	}
	for _, comment := range gd.Doc.List {
		name, ok := parseGoSumTypeAnnotation(comment.Text)
		if ok {
			return name, true
		}
	}
	return "", false
}

// parseGoSumTypeAnnotation parses "//gosumtype:impl Name" and returns "Name".
// The "impl" keyword keeps the value lowercase so gofmt preserves the directive.
func parseGoSumTypeAnnotation(text string) (string, bool) {
	rest, ok := strings.CutPrefix(text, "//gosumtype:impl ")
	if !ok {
		return "", false
	}
	name := strings.TrimSpace(rest)
	if name == "" {
		return "", false
	}
	return name, true
}

func extractSumType(pkg string, gd *ast.GenDecl, ifaceName string, ifaceMap map[string]*ast.TypeSpec) (*sumType, error) {
	ifaceSpec, ok := ifaceMap[ifaceName]
	if !ok {
		return nil, fmt.Errorf("//gosumtype:impl %s references unknown interface %s (must be declared in the same file)", ifaceName, ifaceName)
	}

	markerMethod, err := extractMarkerMethod(ifaceSpec)
	if err != nil {
		return nil, fmt.Errorf("interface %s: %w", ifaceName, err)
	}

	var variantSpecs []*ast.TypeSpec
	for _, spec := range gd.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		variantSpecs = append(variantSpecs, ts)
	}

	if len(variantSpecs) == 0 {
		return nil, nil
	}

	variants := make([]variant, 0, len(variantSpecs))
	for _, vs := range variantSpecs {
		v := variant{name: vs.Name.Name}
		if vs.TypeParams != nil {
			for _, field := range vs.TypeParams.List {
				for _, name := range field.Names {
					v.typeParams = append(v.typeParams, name.Name)
				}
			}
		}
		variants = append(variants, v)
	}

	return &sumType{
		pkg:          pkg,
		ifaceName:    ifaceName,
		markerMethod: markerMethod,
		variants:     variants,
	}, nil
}

func extractMarkerMethod(ts *ast.TypeSpec) (string, error) {
	iface := ts.Type.(*ast.InterfaceType)
	for _, method := range iface.Methods.List {
		if len(method.Names) == 0 {
			continue // embedded interface, not a method
		}
		name := method.Names[0].Name
		if ast.IsExported(name) {
			continue
		}
		ft, ok := method.Type.(*ast.FuncType)
		if !ok {
			continue
		}
		if ft.Params.NumFields() == 0 && (ft.Results == nil || ft.Results.NumFields() == 0) {
			return name, nil
		}
	}
	return "", fmt.Errorf("no unexported zero-param zero-return marker method found")
}
