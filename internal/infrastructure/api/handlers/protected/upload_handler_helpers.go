package protected

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────────

func resolveFolder(ctx string) string {
	if folder, ok := uploadFolders[ctx]; ok {
		return folder
	}
	return "uploads"
}

func buildFilename(folder, ext string) string {
	return fmt.Sprintf("%s/%s%s", folder, uuid.New().String(), ext)
}

func isExtAllowed(ext, category string) bool {
	if category == "" {
		category = "any"
	}
	cat, ok := allowedUploadCategories[category]
	if !ok {
		// Unknown category → fall back to "any"
		cat = allowedUploadCategories["any"]
	}
	for _, allowed := range cat.exts {
		if allowed == ext {
			return true
		}
	}
	// "any" category is the superset — also check there
	if category != "any" {
		anyCat := allowedUploadCategories["any"]
		for _, allowed := range anyCat.exts {
			if allowed == ext {
				return true
			}
		}
	}
	return false
}

func getMimesForCategory(category string) []string {
	if cat, ok := allowedUploadCategories[category]; ok {
		return cat.mimes
	}
	return nil
}

// mimeInList reports whether detectedMime (as returned by http.DetectContentType,
// which may include a "; charset=..." suffix) matches one of allowedMimes.
func mimeInList(detectedMime string, allowedMimes []string) bool {
	if idx := strings.IndexByte(detectedMime, ';'); idx >= 0 {
		detectedMime = detectedMime[:idx]
	}
	detectedMime = strings.TrimSpace(strings.ToLower(detectedMime))
	for _, mime := range allowedMimes {
		if detectedMime == strings.ToLower(mime) {
			return true
		}
	}
	return false
}
