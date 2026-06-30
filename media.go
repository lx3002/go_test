package main

import(
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const MaxUpload_size = 20*1024*1024
const UploadDirectory = "./uploads"


func HandleMediaUpload(w http.ResponseWriter, r http.Request){
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != http.MethodPost{
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxUpload_size)
	if err := r.ParseMultipartForm(MaxUpload_size); err != nil{
		http.Error(w, "filesize size exceeds 200mb limit", http.StatusBadRequest)
		return
	}
	file, handler, err := r.FormFile("media")
	if err != nil{
		http.Error(w, "invalid file key", http.StatusBadRequest)
		return
	}
	defer file.Close()

	buffer := make([] byte, 512)

	if _, err := file.Read(buffer); err != nil{
		http.Error(w, "cannnot read file ", http.StatusInternalServerError)
		return
    }
	file.Seek(0, io.SeekStart)

	contnet_type := http.DetectContentType(buffer)

	 if !strings.HasPrefix(contnet_type, "image/") && !strings.HasPrefix(contnet_type, "video/"){
		http.Error(w, "Prohibited file type. images and videos only", http.StatusUnsupportedMediaType)
    }

	random_Bytes := make([]byte, 16)
    rand.Read(random_Bytes)
	safeFilename := fmt.Sprintf("%x%s", random_Bytes, filepath.Ext(handler.Filename))

	os.MkdirAll(UploadDirectory, os.ModePerm)
	target_path := filepath.Join(UploadDirectory, safeFilename)
	destination_file, err := os.Create(target_path)
	if err != nil{
		http.Error(w, "storage allocation failure", http.StatusInternalServerError)
		return
	}

	defer destination_file.Close()

	if _, err := io.Copy(destination_file, file); err!= nil{
		http.Error(w, "timeout error", http.StatusInternalServerError)
		return
	}

	file_url := fmt.Sprintf("/static/upload/%S", safeFilename)
	w.Header().Set("Content-type", "text/plain")
	w.Write([]byte(file_url))
}