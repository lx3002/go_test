package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoginConfigAndPrivateRoomValidation(t *testing.T) {
	loginRecorder := httptest.NewRecorder()
	handleLogin(loginRecorder, httptest.NewRequest(http.MethodGet, "/login?username=Alice", nil))
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginRecorder.Code, http.StatusOK)
	}

	var loginBody map[string]string
	if err := json.NewDecoder(loginRecorder.Body).Decode(&loginBody); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if loginBody["username"] != "Alice" || loginBody["token"] == "" {
		t.Fatalf("unexpected login response: %#v", loginBody)
	}

	configRecorder := httptest.NewRecorder()
	handleConfig(configRecorder, httptest.NewRequest(http.MethodGet, "/config", nil))
	if configRecorder.Code != http.StatusOK {
		t.Fatalf("config status = %d, want %d", configRecorder.Code, http.StatusOK)
	}

	wsRecorder := httptest.NewRecorder()
	wsURL := "/ws?token=" + url.QueryEscape(loginBody["token"]) + "&room=secret&private=1"
	serveWs(NewHub(), wsRecorder, httptest.NewRequest(http.MethodGet, wsURL, nil))
	if wsRecorder.Code != http.StatusBadRequest {
		t.Fatalf("private websocket without key status = %d, want %d", wsRecorder.Code, http.StatusBadRequest)
	}
}

func TestImageUpload(t *testing.T) {
	token, err := CreateToken("Alice")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	_, statErr := os.Stat(UploadDirectory)
	uploadDirExisted := statErr == nil

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("media", "pixel.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(tinyPNG()); err != nil {
		t.Fatalf("write png: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	HandleMediaUpload(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("upload status = %d, body = %q", recorder.Code, recorder.Body.String())
	}

	var response uploadResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if response.MediaType != "image" || !strings.HasPrefix(response.URL, "/static/uploads/") {
		t.Fatalf("unexpected upload response: %#v", response)
	}

	uploadedFile := filepath.Join(UploadDirectory, filepath.Base(response.URL))
	if err := os.Remove(uploadedFile); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove uploaded file: %v", err)
	}
	if !uploadDirExisted {
		_ = os.Remove(UploadDirectory)
	}
}

func tinyPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x04, 0x00, 0x00, 0x00, 0xb5, 0x1c, 0x0c,
		0x02, 0x00, 0x00, 0x00, 0x0b, 0x49, 0x44, 0x41, 0x54,
		0x78, 0xda, 0x63, 0xfc, 0xff, 0x1f, 0x00, 0x03, 0x03,
		0x02, 0x00, 0xef, 0xbf, 0xa7, 0xdb, 0x00, 0x00, 0x00,
		0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
}
