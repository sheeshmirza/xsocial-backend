package api

import (
	"net/http"
)

func (s *Server) uploadMedia(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB
		writeError(w, http.StatusBadRequest, "failed to parse multipart form: "+err.Error())
		return
	}

	if r.MultipartForm == nil || len(r.MultipartForm.File) == 0 {
		writeError(w, http.StatusBadRequest, "no files provided")
		return
	}

	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		files = r.MultipartForm.File["media"]
	}
	if len(files) == 0 {
		files = r.MultipartForm.File["files"]
	}
	if len(files) == 0 {
		// Take the first available file array from the form
		for _, fileList := range r.MultipartForm.File {
			files = fileList
			break
		}
	}

	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "no valid file field found in request")
		return
	}

	results, err := s.uploadService.ProcessUploads(files, r.Host)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"data":    results,
		"count":   len(results),
	})
}

func (s *Server) getUploadedMedia(w http.ResponseWriter, r *http.Request) {
	mediaID := r.PathValue("id")
	if mediaID == "" {
		writeError(w, http.StatusBadRequest, "missing media id")
		return
	}

	media := s.uploadService.GetMedia(mediaID)
	if media == nil {
		writeError(w, http.StatusNotFound, "media not found or expired")
		return
	}

	w.Header().Set("Content-Type", media.MimeType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(media.Buffer)
}
