package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"xsocial-backend/internal/service"
	"xsocial-backend/internal/utils"
)

type UploadHandler struct {
	uploadService *service.UploadService
}

func NewUploadHandler(uploadService *service.UploadService) *UploadHandler {
	return &UploadHandler{
		uploadService: uploadService,
	}
}

func (h *UploadHandler) UploadMedia(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.Error(utils.BadRequest("Failed to parse multipart form"))
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.Error(utils.BadRequest("No files were uploaded"))
		return
	}

	results, err := h.uploadService.ProcessUploads(files, c.Request.Host)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusCreated, "Files uploaded successfully", results)
}

func (h *UploadHandler) GetUploadedMedia(c *gin.Context) {
	id := c.Param("id")
	entry := h.uploadService.GetMedia(id)
	if entry == nil {
		c.Error(utils.NotFound("Media not found or expired"))
		return
	}

	c.Header("Content-Type", entry.MimeType)
	c.Header("Cache-Control", "private, max-age=3600")
	c.Header("Content-Length", fmt.Sprintf("%d", len(entry.Buffer)))
	c.Data(http.StatusOK, entry.MimeType, entry.Buffer)
}
