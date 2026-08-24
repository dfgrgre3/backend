package db

import (
	"os"
	"regexp"
	"strings"
)

// parsePrismaSchema reads the Prisma schema file at path and returns
// a map of model name → PrismaModel.
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
