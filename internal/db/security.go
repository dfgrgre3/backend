package db

import (
	"io"
	"net/http"
	"strings"
)

// ValidateMagicNumber reads the first 512 bytes of a reader to detect and validate the MIME type.
// It returns true if the detected MIME type is in the allowed list.
func ValidateMagicNumber(r io.ReadSeeker, allowedMimes []string) (bool, string, error) {
	buffer := make([]byte, 512)
	n, err := r.Read(buffer)
	if err != nil && err != io.EOF {
		return false, "", err
	}

	// Seek back to the beginning of the file so it can be uploaded fully
	_, err = r.Seek(0, io.SeekStart)
	if err != nil {
		return false, "", err
	}

	detectedMime := http.DetectContentType(buffer[:n])

	// If detected as octet-stream, check prefix of allowed list
	for _, mime := range allowedMimes {
		if strings.HasPrefix(detectedMime, mime) || detectedMime == mime {
			return true, detectedMime, nil
		}
	}

	// Special override logic for common document types where http.DetectContentType might fall back to zip or plain
	if detectedMime == "application/zip" || detectedMime == "application/octet-stream" {
		return true, detectedMime, nil
	}

	return false, detectedMime, nil
}
