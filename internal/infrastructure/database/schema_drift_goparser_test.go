package db

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

// parseGoModels scans every Go file in dirPath, extracts struct definitions
// with their GORM column tags, and applies any TableName() overrides.
func parseGoModels(dirPath string) (map[string]GoStruct, error) {
	fset := token.NewFileSet()
	//lint:ignore SA1019 ParseDir is intentionally used to preserve package grouping in this schema audit.
	pkgs, err := parser.ParseDir(fset, dirPath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	goStructs := make(map[string]GoStruct)
	tableNameOverrides := make(map[string]string)

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				// 1. Detect Struct Definitions
				typeSpec, ok := n.(*ast.TypeSpec)
				if ok {
					structType, ok := typeSpec.Type.(*ast.StructType)
					if ok {
						structName := typeSpec.Name.Name
						goStruct := GoStruct{
							Name:      structName,
							TableName: structName, // default, might be overridden by TableName()
							Fields:    make(map[string]GoField),
						}

						for _, field := range structType.Fields.List {
							// Go fields can have multiple names (e.g. A, B string)
							var fieldNames []string
							if len(field.Names) == 0 {
								// Embedded field
								switch t := field.Type.(type) {
								case *ast.Ident:
									fieldNames = append(fieldNames, t.Name)
								case *ast.SelectorExpr:
									fieldNames = append(fieldNames, t.Sel.Name)
								}
							} else {
								for _, nameIdent := range field.Names {
									fieldNames = append(fieldNames, nameIdent.Name)
								}
							}

							fieldTypeStr := extractFieldType(field.Type)

							// Parse tag
							var tagValue string
							if field.Tag != nil {
								tagValue = field.Tag.Value
							}

							columnName, isGormIgnored := parseGormTag(tagValue)

							for _, fName := range fieldNames {
								goStruct.Fields[fName] = GoField{
									Name:          fName,
									Type:          fieldTypeStr,
									ColumnName:    columnName,
									IsGormIgnored: isGormIgnored,
								}
							}
						}

						goStructs[structName] = goStruct
					}
				}

				// 2. Detect TableName() methods
				funcDecl, ok := n.(*ast.FuncDecl)
				if ok && funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
					var structName string
					switch t := funcDecl.Recv.List[0].Type.(type) {
					case *ast.Ident:
						structName = t.Name
					case *ast.StarExpr:
						if ident, ok := t.X.(*ast.Ident); ok {
							structName = ident.Name
						}
					}

					if structName != "" && funcDecl.Name.Name == "TableName" {
						// Look for return statement in the method body
						ast.Inspect(funcDecl.Body, func(bodyNode ast.Node) bool {
							retStmt, ok := bodyNode.(*ast.ReturnStmt)
							if ok && len(retStmt.Results) == 1 {
								basicLit, ok := retStmt.Results[0].(*ast.BasicLit)
								if ok && basicLit.Kind == token.STRING {
									tableNameOverrides[structName] = strings.Trim(basicLit.Value, `"`)
								}
							}
							return true
						})
					}
				}

				return true
			})
		}
	}

	// Apply TableName overrides
	for sName, tableName := range tableNameOverrides {
		if s, ok := goStructs[sName]; ok {
			s.TableName = tableName
			goStructs[sName] = s
		}
	}

	return goStructs, nil
}

// extractFieldType converts an ast.Expr into a human-readable type string.
func extractFieldType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return "*" + ident.Name
		}
		if sel, ok := t.X.(*ast.SelectorExpr); ok {
			return "*" + sel.Sel.Name
		}
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.ArrayType:
		eltType := ""
		switch elt := t.Elt.(type) {
		case *ast.Ident:
			eltType = elt.Name
		case *ast.SelectorExpr:
			eltType = elt.Sel.Name
		case *ast.StarExpr:
			if ident, ok := elt.X.(*ast.Ident); ok {
				eltType = ident.Name
			} else if sel, ok := elt.X.(*ast.SelectorExpr); ok {
				eltType = sel.Sel.Name
			}
		}
		return "[]" + eltType
	}
	return ""
}

// parseGormTag extracts the column name and gorm-ignored flag from a struct field tag string.
func parseGormTag(tagValue string) (columnName string, isGormIgnored bool) {
	gormTagRegexp := regexp.MustCompile(`gorm:"([^"]+)"`)
	if matches := gormTagRegexp.FindStringSubmatch(tagValue); len(matches) > 1 {
		tagContent := matches[1]
		parts := strings.Split(tagContent, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "-" {
				isGormIgnored = true
			} else if strings.HasPrefix(part, "column:") {
				columnName = strings.TrimPrefix(part, "column:")
			}
		}
	}
	return columnName, isGormIgnored
}
