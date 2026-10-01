package handler

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
)

// Both resume and final-report uploads validate extension, MIME, and PDF signature.
func openValidatedPDF(header *multipart.FileHeader) (multipart.File, error) {
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(header.Header.Get("Content-Type"), ";")[0]))
	if header.Size <= 0 || !strings.EqualFold(filepath.Ext(header.Filename), ".pdf") || contentType != "application/pdf" {
		return nil, fmt.Errorf("invalid PDF metadata")
	}
	source, err := header.Open()
	if err != nil {
		return nil, err
	}
	signature := make([]byte, 5)
	if _, err := io.ReadFull(source, signature); err != nil || string(signature) != "%PDF-" {
		source.Close()
		return nil, fmt.Errorf("invalid PDF signature")
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		source.Close()
		return nil, err
	}
	return source, nil
}

func removeUploadedFile(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("remove uploaded file %q: %v", path, err)
	}
}
