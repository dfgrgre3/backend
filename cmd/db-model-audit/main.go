// Command db-model-audit extracts the DB columns implied by every GORM model
// (struct with an explicit TableName method) in a Go package tree and prints
// them as "table\tcolumn" lines. It is the "expected schema" half of a drift
// check; pipe/compare the output against information_schema to find columns a
// model references that no migration ever created.
//
//	usage: go run ./cmd/db-model-audit -dir internal/domain/common
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type fieldInfo struct {
	name string // struct field name (for default DB name)
	tag  string // full gorm struct tag, e.g. `gorm:"column:x;not null"`
}

func main() {
	dir := flag.String("dir", "internal/domain/common", "directory tree to scan for model files")
	flag.Parse()

	tableColumns := map[string]map[string]bool{} // table -> set of columns
	// struct type name -> fields
	structFields := map[string][]fieldInfo{}
	// struct type name -> explicit TableName literal
	tableNames := map[string]string{}

	err := filepath.Walk(*dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}
		// First pass over declarations in this file: struct types + TableName methods.
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
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				fields := make([]fieldInfo, 0, len(st.Fields.List))
				for _, fld := range st.Fields.List {
					if len(fld.Names) == 0 {
						continue // embedded type
					}
					fi := fieldInfo{name: fld.Names[0].Name}
					if fld.Tag != nil {
						fi.tag = fld.Tag.Value
					}
					fields = append(fields, fi)
				}
				structFields[ts.Name.Name] = fields
			}
		}
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Name.Name != "TableName" || fd.Recv == nil || len(fd.Recv.List) != 1 || fd.Type.Results == nil {
				continue
			}
			if len(fd.Type.Results.List) != 1 {
				continue
			}
			typeName := receiverTypeName(fd.Recv.List[0].Type)
			if typeName == "" {
				continue
			}
			lit, ok := fd.Type.Results.List[0].Type.(*ast.Ident)
			if !ok || lit.Name != "string" {
				continue
			}
			// Locate the single returned string literal in the body.
			ret := stringReturnLiteral(fd.Body)
			if ret != "" {
				tableNames[typeName] = ret
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan failed:", err)
		os.Exit(1)
	}

	for typeName, table := range tableNames {
		cols := tableColumns[table]
		if cols == nil {
			cols = map[string]bool{}
			tableColumns[table] = cols
		}
		for _, fld := range structFields[typeName] {
			col, skip := columnFor(fld)
			if skip {
				continue
			}
			cols[col] = true
		}
	}

	tables := make([]string, 0, len(tableColumns))
	for t := range tableColumns {
		tables = append(tables, t)
	}
	sort.Strings(tables)
	for _, t := range tables {
		cols := make([]string, 0, len(tableColumns[t]))
		for c := range tableColumns[t] {
			cols = append(cols, c)
		}
		sort.Strings(cols)
		for _, c := range cols {
			fmt.Printf("%s\t%s\n", t, c)
		}
	}
}

// receiverTypeName resolves the type name of a method receiver
// (handles both value and pointer receivers).
func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return ""
}

// stringReturnLiteral returns the first plain string literal returned by body,
// used to read `func (X) TableName() string { return "tbl" }`.
func stringReturnLiteral(body *ast.BlockStmt) string {
	if body == nil {
		return ""
	}
	for _, stmt := range body.List {
		rs, ok := stmt.(*ast.ReturnStmt)
		if !ok || len(rs.Results) != 1 {
			continue
		}
		if bl, ok := rs.Results[0].(*ast.BasicLit); ok && bl.Kind == token.STRING {
			s := strings.Trim(bl.Value, "`\"")
			return s
		}
	}
	return ""
}

// columnFor maps a struct field to its DB column name following the same
// rules GORM uses: explicit column: tag wins; a `gorm:"-"` field is skipped;
// otherwise the field name is converted with GORM's naming strategy.
func columnFor(f fieldInfo) (string, bool) {
	tag := strings.Trim(f.tag, "`")
	if tag == "" {
		return toDBName(f.name), false
	}
	meta := gormMeta(tag)
	if meta.skip {
		return "", true
	}
	if meta.column != "" {
		return meta.column, false
	}
	return toDBName(f.name), false
}

// commonInitialisms mirrors golint's list; GORM rewrites these to title case
// before snake_casing so "UserID" -> user_id rather than user_i_d.
var commonInitialisms = []string{
	"API", "ASCII", "CPU", "CSS", "DNS", "EOF", "GUID", "HTML", "HTTP",
	"HTTPS", "ID", "IP", "JSON", "LHS", "QPS", "RAM", "RHS", "RPC", "SLA",
	"SMTP", "SQL", "SSH", "TCP", "TLS", "TTL", "UDP", "UI", "UID", "UUID",
	"URI", "URL", "UTF8", "VM", "XML", "XMPP", "XSRF", "XSS",
}

// toDBName reproduces gorm.io/gorm/schema.NamingStrategy.toDBName behavior
// (common-initialism rewriting + case-transition splitting + lowercasing).
func toDBName(name string) string {
	if name == "" {
		return ""
	}
	value := name
	for _, abbr := range commonInitialisms {
		title := string(abbr[0]) + strings.ToLower(abbr[1:])
		value = strings.ReplaceAll(value, abbr, title)
	}
	if len(value) == 0 {
		return ""
	}
	buf := make([]byte, 0, len(value)+3)
	var lastCase, nextCase, nextNumber bool
	for i := 0; i < len(value)-1; i++ {
		curCase := value[i] >= 'A' && value[i] <= 'Z'
		nextCase = value[i+1] >= 'A' && value[i+1] <= 'Z'
		nextNumber = value[i+1] >= '0' && value[i+1] <= '9'
		if curCase {
			if lastCase && (nextCase || nextNumber) {
				// continuation of an acronym run (e.g. the "I" of "ID")
				buf = append(buf, value[i]+('a'-'A'))
			} else {
				if i > 0 && value[i-1] != '_' && value[i+1] != '_' {
					buf = append(buf, '_')
				}
				buf = append(buf, value[i]+('a'-'A'))
			}
		} else {
			buf = append(buf, value[i])
		}
		lastCase = curCase
	}
	buf = append(buf, value[len(value)-1])
	return string(buf)
}

type gormTagMeta struct {
	column string
	skip   bool
}

// gormMeta parses the gorm:"..." struct-tag value (the innermost quoted part).
func gormMeta(tag string) gormTagMeta {
	// tag looks like: gorm:"column:x;type:uuid;not null" json:"..."
	start := strings.Index(tag, "gorm:")
	if start < 0 {
		return gormTagMeta{}
	}
	rest := tag[start+len("gorm:"):]
	if rest == "" || rest[0] != '"' {
		return gormTagMeta{}
	}
	end := strings.Index(rest[1:], `"`)
	if end < 0 {
		return gormTagMeta{}
	}
	val := rest[1 : 1+end]
	meta := gormTagMeta{}
	for _, part := range strings.Split(val, ";") {
		part = strings.TrimSpace(part)
		if part == "-" {
			meta.skip = true
			continue
		}
		if strings.HasPrefix(part, "column:") {
			meta.column = strings.TrimSpace(strings.TrimPrefix(part, "column:"))
		}
	}
	return meta
}
