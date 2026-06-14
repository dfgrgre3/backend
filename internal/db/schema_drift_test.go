package db

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type PrismaField struct {
	Name       string
	Type       string
	IsArray    bool
	IsOptional bool
	ColumnName string // from @map("...")
	IsRelation bool
}

type PrismaModel struct {
	Name      string
	TableName string // from @@map("...")
	Fields    map[string]PrismaField
}

type GoField struct {
	Name       string
	Type       string
	ColumnName string // from gorm:"column:..."
	IsGormIgnored bool // e.g. gorm:"-"
}

type GoStruct struct {
	Name      string
	TableName string
	Fields    map[string]GoField
}

// Helper to convert camelCase/PascalCase to snake_case (GORM default naming)
func toSnakeCase(str string) string {
	var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

func parsePrismaSchema(path string) (map[string]PrismaModel, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	models := make(map[string]PrismaModel)
	lines := strings.Split(string(content), "\n")

	var currentModel *PrismaModel
	modelHeaderRegexp := regexp.MustCompile(`^model\s+(\w+)\s*\{`)
	mapRegexp := regexp.MustCompile(`@@map\("([^"]+)"\)`)
	fieldMapRegexp := regexp.MustCompile(`@map\("([^"]+)"\)`)
	relationRegexp := regexp.MustCompile(`@relation\(`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "///") {
			continue
		}

		// Check model start
		if matches := modelHeaderRegexp.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			currentModel = &PrismaModel{
				Name:      name,
				TableName: name, // Default is model name
				Fields:    make(map[string]PrismaField),
			}
			continue
		}

		if line == "}" {
			if currentModel != nil {
				models[currentModel.Name] = *currentModel
				currentModel = nil
			}
			continue
		}

		if currentModel != nil {
			// Check @@map
			if matches := mapRegexp.FindStringSubmatch(line); len(matches) > 1 {
				currentModel.TableName = matches[1]
				continue
			}

			// It's a field: name type [attributes]
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				fieldName := parts[0]
				fieldTypeRaw := parts[1]

				// Ignore block attributes
				if strings.HasPrefix(fieldName, "@@") {
					continue
				}

				isArray := strings.HasSuffix(fieldTypeRaw, "[]")
				isOptional := strings.HasSuffix(fieldTypeRaw, "?")

				baseType := fieldTypeRaw
				if isArray {
					baseType = strings.TrimSuffix(baseType, "[]")
				}
				if isOptional {
					baseType = strings.TrimSuffix(baseType, "?")
				}

				// Check map/relation attribute in the rest of the line
				rest := strings.Join(parts[2:], " ")
				columnName := ""
				if matches := fieldMapRegexp.FindStringSubmatch(rest); len(matches) > 1 {
					columnName = matches[1]
				}

				isRelation := relationRegexp.MatchString(rest)

				currentModel.Fields[fieldName] = PrismaField{
					Name:       fieldName,
					Type:       baseType,
					IsArray:    isArray,
					IsOptional: isOptional,
					ColumnName: columnName,
					IsRelation: isRelation,
				}
			}
		}
	}

	// Post-process to mark fields whose type is another model as relations
	for _, m := range models {
		for fName, f := range m.Fields {
			if _, exists := models[f.Type]; exists {
				f.IsRelation = true
				m.Fields[fName] = f
			}
		}
	}

	return models, nil
}

func parseGoModels(dirPath string) (map[string]GoStruct, error) {
	fset := token.NewFileSet()
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

							// Extract field type string
							fieldTypeStr := ""
							switch t := field.Type.(type) {
							case *ast.Ident:
								fieldTypeStr = t.Name
							case *ast.StarExpr:
								if ident, ok := t.X.(*ast.Ident); ok {
									fieldTypeStr = "*" + ident.Name
								} else if sel, ok := t.X.(*ast.SelectorExpr); ok {
									fieldTypeStr = "*" + sel.Sel.Name
								}
							case *ast.SelectorExpr:
								fieldTypeStr = t.Sel.Name
							case *ast.ArrayType:
								fieldTypeStr = "[]"
							}

							// Parse tag
							var tagValue string
							if field.Tag != nil {
								tagValue = field.Tag.Value
							}

							columnName := ""
							isGormIgnored := false
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

// Verify that types are compatible
func areTypesCompatible(prismaType, goType string) bool {
	// Clean up pointer prefixes or packages from Go types
	goTypeClean := strings.TrimPrefix(goType, "*")
	goTypeClean = strings.TrimPrefix(goTypeClean, "models.")

	switch prismaType {
	case "String":
		// string, uuid.UUID, custom enums or arrays/types
		return goTypeClean == "string" || goTypeClean == "UUID" || strings.Contains(goTypeClean, "Role") || strings.Contains(goTypeClean, "Status") || goTypeClean == "JSONStringArray"
	case "Int":
		return goTypeClean == "int" || goTypeClean == "int32" || goTypeClean == "int64"
	case "Float", "Decimal":
		return goTypeClean == "float32" || goTypeClean == "float64"
	case "Boolean":
		return goTypeClean == "bool"
	case "DateTime":
		return goTypeClean == "Time" // time.Time
	case "Json":
		return goTypeClean == "json.RawMessage" || goTypeClean == "RawMessage" || goTypeClean == "string" || goTypeClean == "JSONStringArray" || goTypeClean == "JSON" || goTypeClean == "[]byte"
	}
	// Fallback to true if we cannot analyze it fully, to avoid false positives
	return true
}

func TestSchemaDrift(t *testing.T) {
	// Find paths relatively
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}

	// Paths relative to internal/db
	prismaPath := filepath.Join(wd, "..", "..", "prisma", "schema.prisma")
	modelsDir := filepath.Join(wd, "..", "models")

	if _, err := os.Stat(prismaPath); os.IsNotExist(err) {
		t.Skipf("schema.prisma not found at %s. Skipping drift check.", prismaPath)
		return
	}

	prismaModels, err := parsePrismaSchema(prismaPath)
	if err != nil {
		t.Fatalf("Failed to parse Prisma schema: %v", err)
	}

	goStructs, err := parseGoModels(modelsDir)
	if err != nil {
		t.Fatalf("Failed to parse Go models: %v", err)
	}

	// Map Go models by TableName for easy lookup
	goModelsByTable := make(map[string]GoStruct)
	for _, s := range goStructs {
		goModelsByTable[s.TableName] = s
	}

	driftCount := 0

	for _, pm := range prismaModels {
		t.Run(fmt.Sprintf("Model_%s", pm.Name), func(t *testing.T) {
			// Find matching Go struct by table name or model name
			gs, ok := goModelsByTable[pm.TableName]
			if !ok {
				gs, ok = goStructs[pm.Name]
			}

			if !ok {
				t.Errorf("Prisma model %q (table: %q) has no corresponding Go struct in internal/models/", pm.Name, pm.TableName)
				driftCount++
				return
			}

			// Check each field in the Prisma model
			for _, pf := range pm.Fields {
				// Skip relations (navigation properties) as they aren't physical database columns in the same struct
				if pf.IsRelation {
					continue
				}

				// Find corresponding field in Go struct
				var foundField *GoField
				expectedColumnName := pf.ColumnName
				if expectedColumnName == "" {
					expectedColumnName = toSnakeCase(pf.Name)
				}

				for _, gf := range gs.Fields {
					if gf.IsGormIgnored {
						continue
					}

					// A field matches if:
					// 1. Explicit GORM column name matches
					// 2. Or the field name matches (case-insensitive)
					// 3. Or the snake_case of the field name matches the expected column name
					if gf.ColumnName == expectedColumnName ||
						strings.EqualFold(gf.Name, pf.Name) ||
						toSnakeCase(gf.Name) == expectedColumnName {
						foundField = &gf
						break
					}
				}

				if foundField == nil {
					t.Errorf("Prisma field %s.%s (column: %q) is missing in Go struct %s", pm.Name, pf.Name, expectedColumnName, gs.Name)
					driftCount++
					continue
				}

				// Check type compatibility
				if !areTypesCompatible(pf.Type, foundField.Type) {
					t.Errorf("Type mismatch on %s.%s (Go struct %s.%s): Prisma type %q is incompatible with Go type %q",
						pm.Name, pf.Name, gs.Name, foundField.Name, pf.Type, foundField.Type)
					driftCount++
				}
			}
		})
	}

	if driftCount > 0 {
		t.Fatalf("Detected %d schema structural drift(s) between Prisma schema and GORM models.", driftCount)
	} else {
		t.Log("No schema structural drift detected.")
	}
}
