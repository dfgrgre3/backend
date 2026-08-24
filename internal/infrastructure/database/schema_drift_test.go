package db

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSchemaDrift verifies that every Go struct in the models package
// has a corresponding table in the Prisma schema and that all mapped
// columns exist with compatible types.
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

	driftCount := 0

	for _, gs := range goStructs {
		// Skip helper/custom types that aren't database tables
		if gs.Name == "JSONStringArray" {
			continue
		}

		t.Run(fmt.Sprintf("Model_%s", gs.Name), func(t *testing.T) {
			// Find matching Prisma model by TableName or Name
			var pm *PrismaModel
			for _, m := range prismaModels {
				if m.TableName == gs.TableName || m.Name == gs.TableName || m.TableName == gs.Name || m.Name == gs.Name {
					pm = &m
					break
				}
			}

			if pm == nil {
				t.Errorf("Go struct %s (table: %q) has no corresponding database table in Prisma schema", gs.Name, gs.TableName)
				driftCount++
				return
			}

			// Check each field in the Go struct
			for _, gf := range gs.Fields {
				// Skip fields ignored by GORM
				if gf.IsGormIgnored {
					continue
				}

				// Skip relations. A field is a relation if its type (excluding pointer * and slice [] prefixes)
				// matches a defined Go struct in the models package.
				cleanType := strings.TrimPrefix(gf.Type, "*")
				cleanType = strings.TrimPrefix(cleanType, "[]")
				if _, exists := goStructs[cleanType]; exists {
					continue
				}

				// Determine the column name we expect in the database
				expectedColumnName := gf.ColumnName
				if expectedColumnName == "" {
					expectedColumnName = toSnakeCase(gf.Name)
				}

				// Find corresponding field in the database table
				var foundField *PrismaField
				for _, pf := range pm.Fields {
					pfColumnName := pf.ColumnName
					if pfColumnName == "" {
						pfColumnName = pf.Name
					}

					if pfColumnName == expectedColumnName ||
						strings.EqualFold(pf.Name, gf.Name) ||
						toSnakeCase(pf.Name) == expectedColumnName {
						foundField = &pf
						break
					}
				}

				if foundField == nil {
					t.Errorf("Go field %s.%s (expected column: %q) is missing in DB table %q",
						gs.Name, gf.Name, expectedColumnName, pm.TableName)
					driftCount++
					continue
				}

				// Check type compatibility
				if !areTypesCompatible(foundField.Type, gf.Type) {
					t.Errorf("Type mismatch on %s.%s (DB column %q has type %q, Go field has type %q)",
						gs.Name, gf.Name, foundField.Name, foundField.Type, gf.Type)
					driftCount++
				}
			}
		})
	}

	if driftCount > 0 {
		t.Fatalf("Detected %d schema structural drift(s) between GORM models and Prisma schema.", driftCount)
	} else {
		t.Log("No schema structural drift detected.")
	}
}
