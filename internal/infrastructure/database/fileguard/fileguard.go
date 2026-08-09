package fileguard

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

const mimeSniffBytes = 8192

var openXMLMIMEs = map[string]string{
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   "word/",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         "xl/",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": "ppt/",
}

// ValidateMagicNumber validates file contents against an explicit allow-list.
// Generic ZIP and octet-stream detections are never accepted on their own.
func ValidateMagicNumber(r io.ReadSeeker, allowedMimes []string) (bool, string, error) {
	buffer := make([]byte, mimeSniffBytes)
	n, err := r.Read(buffer)
	if err != nil && err != io.EOF {
		return false, "", err
	}

	// Seek back to the beginning of the file so it can be uploaded fully
	_, err = r.Seek(0, io.SeekStart)
	if err != nil {
		return false, "", err
	}

	detectedMime := normalizeMIME(http.DetectContentType(buffer[:n]))
	if detectedMime == "application/zip" {
		return validateOpenXML(r, allowedMimes)
	}
	if detectedMime == "application/octet-stream" {
		return false, detectedMime, nil
	}

	for _, mime := range allowedMimes {
		if detectedMime == normalizeMIME(mime) {
			return true, detectedMime, nil
		}
	}

	return false, detectedMime, nil
}

func normalizeMIME(value string) string {
	if index := strings.IndexByte(value, ';'); index >= 0 {
		value = value[:index]
	}
	return strings.TrimSpace(strings.ToLower(value))
}

func validateOpenXML(r io.ReadSeeker, allowedMimes []string) (bool, string, error) {
	end, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return false, "application/zip", err
	}
	if _, err = r.Seek(0, io.SeekStart); err != nil {
		return false, "application/zip", err
	}

	readerAt, ok := r.(io.ReaderAt)
	if !ok {
		_, _ = r.Seek(0, io.SeekStart)
		return false, "application/zip", nil
	}
	archive, err := zip.NewReader(readerAt, end)
	if err != nil {
		_, _ = r.Seek(0, io.SeekStart)
		return false, "application/zip", nil
	}

	allowed := make(map[string]struct{}, len(allowedMimes))
	for _, mime := range allowedMimes {
		allowed[normalizeMIME(mime)] = struct{}{}
	}

	for mime, requiredPrefix := range openXMLMIMEs {
		if _, ok := allowed[mime]; !ok {
			continue
		}
		if archiveHasOpenXMLType(archive, mime, requiredPrefix) {
			_, _ = r.Seek(0, io.SeekStart)
			return true, mime, nil
		}
	}

	_, _ = r.Seek(0, io.SeekStart)
	return false, "application/zip", nil
}

func archiveHasOpenXMLType(archive *zip.Reader, mime, requiredPrefix string) bool {
	hasContentTypes := false
	hasRequiredPart := false

	for _, file := range archive.File {
		name := filepath.ToSlash(file.Name)
		if name == "[Content_Types].xml" {
			hasContentTypes = zipFileContains(file, []byte(mime))
		}
		if strings.HasPrefix(name, requiredPrefix) {
			hasRequiredPart = true
		}
	}

	return hasContentTypes && hasRequiredPart
}

func zipFileContains(file *zip.File, needle []byte) bool {
	reader, err := file.Open()
	if err != nil {
		return false
	}
	defer reader.Close()

	contents, err := io.ReadAll(io.LimitReader(reader, 1<<20))
	return err == nil && bytes.Contains(contents, needle)
}
