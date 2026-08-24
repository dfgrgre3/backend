package db

import "strings"

// areTypesCompatible reports whether a Prisma field type and a Go field type
// are considered compatible for schema-drift purposes.
func areTypesCompatible(prismaType, goType string) bool {
	// Clean up pointer prefixes or packages from Go types
	goTypeClean := strings.TrimPrefix(goType, "*")
	goTypeClean = strings.TrimPrefix(goTypeClean, "models.")

	switch prismaType {
	case "String":
		return goTypeClean == "string" ||
			goTypeClean == "UUID" ||
			strings.Contains(goTypeClean, "Role") ||
			strings.Contains(goTypeClean, "Status") ||
			strings.Contains(goTypeClean, "Type") ||
			strings.Contains(goTypeClean, "Level") ||
			strings.Contains(goTypeClean, "Method") ||
			strings.Contains(goTypeClean, "Interval") ||
			goTypeClean == "JSONStringArray" ||
			strings.HasPrefix(goTypeClean, "[]") ||
			goTypeClean == "PGStringArray" ||
			goTypeClean == "StringArray"
	case "Int":
		return goTypeClean == "int" || goTypeClean == "int32" || goTypeClean == "int64" || goTypeClean == "uint" || goTypeClean == "uint32" || goTypeClean == "uint64"
	case "Float", "Decimal":
		return goTypeClean == "float32" || goTypeClean == "float64" || goTypeClean == "int" || goTypeClean == "int32" || goTypeClean == "int64"
	case "Boolean":
		return goTypeClean == "bool"
	case "DateTime":
		return goTypeClean == "Time" || goTypeClean == "DeletedAt"
	case "Json":
		return goTypeClean == "json.RawMessage" ||
			goTypeClean == "RawMessage" ||
			goTypeClean == "string" ||
			goTypeClean == "JSONStringArray" ||
			goTypeClean == "StringArray" ||
			goTypeClean == "PGStringArray" ||
			goTypeClean == "JSON" ||
			goTypeClean == "JSONMap" ||
			goTypeClean == "[]byte"
	}
	// Fallback to true if we cannot analyze it fully, to avoid false positives
	return true
}
