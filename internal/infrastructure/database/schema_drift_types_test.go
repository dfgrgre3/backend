package db

import "gorm.io/gorm/schema"

// PrismaField represents a field parsed from a Prisma schema model.
type PrismaField struct {
	Name       string
	Type       string
	IsArray    bool
	IsOptional bool
	ColumnName string // from @map("...")
	IsRelation bool
}

// PrismaModel represents a model parsed from the Prisma schema.
type PrismaModel struct {
	Name      string
	TableName string // from @@map("...")
	Fields    map[string]PrismaField
}

// GoField represents a field parsed from a Go struct.
type GoField struct {
	Name          string
	Type          string
	ColumnName    string // from gorm:"column:..."
	IsGormIgnored bool   // e.g. gorm:"-"
}

// GoStruct represents a Go struct that maps to a database table.
type GoStruct struct {
	Name      string
	TableName string
	Fields    map[string]GoField
}

var ns = schema.NamingStrategy{}

// toSnakeCase converts camelCase/PascalCase to snake_case using GORM's default naming strategy.
func toSnakeCase(str string) string {
	return ns.ColumnName("", str)
}
