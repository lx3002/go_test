package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const MaxUploadSize = 20 * 1024 * 1024
const UploadDirectory = "./uploads"

type uploadResponse struct {
	URL       string `json:"url"`
	MediaType string `json:"media_type"`
}

func HandleMediaUpload(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if handleOptions(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if _, err := VerifyToken(tokenFromRequest(r)); err != nil {
		http.Error(w, "unauthorized_access", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadSize)
	if err := r.ParseMultipartForm(MaxUploadSize); err != nil {
		http.Error(w, "file size exceeds 20mb limit", http.StatusBadRequest)
		return
	}
	file, handler, err := r.FormFile("media")
	if err != nil {
		http.Error(w, "invalid file key", http.StatusBadRequest)
		return
	}
	defer file.Close()

	buffer := make([]byte, 512)
	bytesRead, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		http.Error(w, "cannot read file", http.StatusInternalServerError)
		return
	}
	if bytesRead == 0 {
		http.Error(w, "empty file", http.StatusBadRequest)
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		http.Error(w, "cannot rewind file", http.StatusInternalServerError)
		return
	}

	contentType := http.DetectContentType(buffer[:bytesRead])
	mediaType := ""
	switch {
	case strings.HasPrefix(contentType, "image/"):
		mediaType = "image"
	case strings.HasPrefix(contentType, "video/"):
		mediaType = "video"
	default:
		http.Error(w, "prohibited file type; images and videos only", http.StatusUnsupportedMediaType)
		return
	}

	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		http.Error(w, "could not generate filename", http.StatusInternalServerError)
		return
	}
	safeFilename := fmt.Sprintf("%x%s", randomBytes, strings.ToLower(filepath.Ext(handler.Filename)))

	if err := os.MkdirAll(UploadDirectory, 0755); err != nil {
		http.Error(w, "storage allocation failure", http.StatusInternalServerError)
		return
	}
	targetPath := filepath.Join(UploadDirectory, safeFilename)
	destinationFile, err := os.Create(targetPath)
	if err != nil {
		http.Error(w, "storage allocation failure", http.StatusInternalServerError)
		return
	}
	defer destinationFile.Close()

	if _, err := io.Copy(destinationFile, file); err != nil {
		http.Error(w, "timeout error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(uploadResponse{
		URL:       fmt.Sprintf("/static/uploads/%s", safeFilename),
		MediaType: mediaType,
	})
}
