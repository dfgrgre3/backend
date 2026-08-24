package protected

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"thanawy-backend/internal/infrastructure/storage"
)

func getChunkMetadata(ctx context.Context, uploadID string) (chunkedChunkMetadata, error) {
	var meta chunkedChunkMetadata
	rc, err := storage.GlobalStorage.Download(ctx, fmt.Sprintf("temp/%s/meta.json", uploadID))
	if err != nil {
		return meta, err
	}
	defer rc.Close()
	val, err := io.ReadAll(rc)
	if err != nil {
		return meta, err
	}
	if err := json.Unmarshal(val, &meta); err != nil {
		return chunkedChunkMetadata{}, fmt.Errorf("corrupted upload session metadata: %w", err)
	}
	return meta, nil
}

func getSortedChunkEntries(ctx context.Context, uploadID string) ([]chunkedChunkEntry, error) {
	prefix := fmt.Sprintf("temp/%s/", uploadID)
	files, err := storage.GlobalStorage.List(ctx, prefix)
	if err != nil {
		return nil, err
	}

	var entries []chunkedChunkEntry
	for _, path := range files {
		parts := strings.Split(path, "/")
		if len(parts) < 3 {
			continue
		}
		idx, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			continue
		}
		entries = append(entries, chunkedChunkEntry{index: idx, path: path})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].index < entries[j].index
	})
	return entries, nil
}

// streamReader reads multiple chunk files sequentially as one stream
type streamReader struct {
	chunks        []chunkedChunkEntry
	currentIdx    int
	currentReader io.ReadCloser
	downloadFn    func(path string) (io.ReadCloser, error)
}

func (r *streamReader) Read(p []byte) (int, error) {
	for {
		if r.currentReader == nil {
			if r.currentIdx >= len(r.chunks) {
				return 0, io.EOF
			}
			rc, err := r.downloadFn(r.chunks[r.currentIdx].path)
			if err != nil {
				return 0, fmt.Errorf("failed to open chunk %d: %w", r.currentIdx, err)
			}
			r.currentReader = rc
		}
		n, err := r.currentReader.Read(p)
		if err == io.EOF {
			_ = r.currentReader.Close()
			r.currentReader = nil
			r.currentIdx++
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (r *streamReader) Close() error {
	if r.currentReader != nil {
		err := r.currentReader.Close()
		r.currentReader = nil
		return err
	}
	return nil
}

func assembleChunkedFile(ctx context.Context, finalFilename string, meta chunkedChunkMetadata, chunks []chunkedChunkEntry, category, ext string) (string, error) {
	stream := &streamReader{
		chunks: chunks,
		downloadFn: func(path string) (io.ReadCloser, error) {
			return storage.GlobalStorage.Download(ctx, path)
		},
	}
	defer stream.Close()

	// Sniff the leading bytes of the assembled stream against the declared
	// category's allowed MIME types before committing the file to permanent
	// storage. This is a lighter check than db.ValidateMagicNumber (no
	// seek-back / OpenXML zip inspection, since chunked uploads are streamed
	// and not fully buffered) but still blocks the common case of a spoofed
	// extension hiding an executable or unexpected binary payload.
	if ext != ".svg" && ext != ".txt" {
		if allowedMimes := getMimesForCategory(category); len(allowedMimes) > 0 {
			sniffBuf := make([]byte, 512)
			n, readErr := io.ReadFull(stream, sniffBuf)
			if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
				return "", fmt.Errorf("failed to read assembled file for validation: %w", readErr)
			}
			sniffBuf = sniffBuf[:n]
			detectedMime := http.DetectContentType(sniffBuf)
			if detectedMime == "application/octet-stream" || !mimeInList(detectedMime, allowedMimes) {
				return "", fmt.Errorf("file content does not match declared type (detected: %s)", detectedMime)
			}
			// Replay the sniffed prefix ahead of the remaining stream.
			stream2 := io.MultiReader(bytes.NewReader(sniffBuf), stream)
			url, err := storage.GlobalStorage.Upload(ctx, finalFilename, stream2, meta.FileSize, meta.FileType)
			if err != nil {
				return "", fmt.Errorf("upload failed: %w", err)
			}
			return url, nil
		}
	}

	url, err := storage.GlobalStorage.Upload(ctx, finalFilename, stream, meta.FileSize, meta.FileType)
	if err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}
	return url, nil
}

func cleanupChunkSession(uploadID string, chunks []chunkedChunkEntry) {
	bgCtx := context.Background()
	for _, chunk := range chunks {
		_ = storage.GlobalStorage.Delete(bgCtx, chunk.path)
	}
	_ = storage.GlobalStorage.Delete(bgCtx, fmt.Sprintf("temp/%s/meta.json", uploadID))
}
