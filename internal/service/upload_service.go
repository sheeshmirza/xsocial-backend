package service

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/utils"
)

type StoredMedia struct {
	Buffer    []byte
	MimeType  string
	ExpiresAt time.Time
}

type UploadedMediaResponse struct {
	Name       string  `json:"name"`
	MimeType   string  `json:"mimeType"`
	Size       float64 `json:"size"`
	URL        string  `json:"url"`
	Base64Data string  `json:"base64Data"`
	DataURI    string  `json:"dataUri"`
}

type UploadService struct {
	config *config.Config
	store  sync.Map
}

func NewUploadService(cfg *config.Config) *UploadService {
	s := &UploadService{
		config: cfg,
	}

	// Periodic cleanup every 10 minutes
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		for range ticker.C {
			now := time.Now()
			s.store.Range(func(key, value any) bool {
				if entry, ok := value.(StoredMedia); ok {
					if now.After(entry.ExpiresAt) {
						s.store.Delete(key)
					}
				}
				return true
			})
		}
	}()

	return s
}

func (s *UploadService) SaveMedia(id string, buffer []byte, mimeType string) {
	s.store.Store(id, StoredMedia{
		Buffer:    buffer,
		MimeType:  mimeType,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})
}

func (s *UploadService) GetMedia(id string) *StoredMedia {
	val, ok := s.store.Load(id)
	if !ok {
		return nil
	}

	entry := val.(StoredMedia)
	if time.Now().After(entry.ExpiresAt) {
		s.store.Delete(id)
		return nil
	}
	return &entry
}

func (s *UploadService) ProcessUploads(files []*multipart.FileHeader, requestHost string) ([]UploadedMediaResponse, error) {
	if len(files) == 0 {
		return nil, utils.BadRequest("No files were uploaded")
	}

	baseURL := strings.TrimRight(s.config.PublicBaseURL, "/")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://%s", requestHost)
	}

	apiPrefix := strings.TrimRight(s.config.ApiPrefix, "/")
	results := make([]UploadedMediaResponse, 0, len(files))

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			return nil, utils.BadRequest(fmt.Sprintf("Failed to open file %s", fileHeader.Filename))
		}

		const maxMediaFileSize = 25 * 1024 * 1024 // 25 MB
		if fileHeader.Size > maxMediaFileSize {
			return nil, utils.BadRequest(fmt.Sprintf("File %s exceeds maximum allowed size of 25MB", fileHeader.Filename))
		}

		buffer, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil {
			return nil, utils.InternalServerError("Failed to read uploaded file")
		}

		mimeType := fileHeader.Header.Get("Content-Type")
		detected := http.DetectContentType(buffer)
		if mimeType == "" || mimeType == "application/octet-stream" {
			mimeType = detected
		}

		mediaID := uuid.New().String()
		encoded := base64.StdEncoding.EncodeToString(buffer)
		dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)

		base64Data := ""
		if strings.HasPrefix(strings.ToLower(mimeType), "image/") {
			base64Data = dataURI
		}

		s.SaveMedia(mediaID, buffer, mimeType)

		fetchableURL := fmt.Sprintf("%s%s/uploads/media/%s", baseURL, apiPrefix, mediaID)

		results = append(results, UploadedMediaResponse{
			Name:       fileHeader.Filename,
			MimeType:   mimeType,
			Size:       float64(fileHeader.Size),
			URL:        fetchableURL,
			Base64Data: base64Data,
			DataURI:    dataURI,
		})
	}

	return results, nil
}
