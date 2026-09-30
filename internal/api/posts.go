package api

import (
	"fmt"
	"net/http"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
)

func (s *Server) createPost(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input models.CreatePostInput
	if !decode(w, r, 0, &input) {
		return
	}

	post, err := s.postService.CreatePost(r.Context(), userOID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, post.BrandID, currentUserID(r), "post.create", "post", post.ID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, post)
}

func (s *Server) listPosts(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	pageNum, limitNum := page(r)
	skip := (pageNum - 1) * limitNum

	queryParams := make(map[string]string)
	for k, v := range r.URL.Query() {
		if len(v) > 0 && k != "page" && k != "limit" {
			queryParams[k] = v[0]
		}
	}

	sort := bson.D{{Key: "createdAt", Value: -1}}
	posts, total, err := s.postService.ListPosts(r.Context(), userOID, queryParams, skip, limitNum, sort)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list posts: "+err.Error())
		return
	}
	if posts == nil {
		posts = []models.Post{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  posts,
		"page":  pageNum,
		"limit": limitNum,
		"total": total,
	})
}

func (s *Server) getPost(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	postOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid post ID")
		return
	}

	post, err := s.postService.GetPost(r.Context(), userOID, postOID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, post)
}

func (s *Server) updatePost(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	postOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid post ID")
		return
	}

	var input models.UpdatePostInput
	if !decode(w, r, 0, &input) {
		return
	}

	post, err := s.postService.UpdatePost(r.Context(), userOID, postOID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, post.BrandID, currentUserID(r), "post.update", "post", postOID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, post)
}

func (s *Server) deletePost(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	postOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid post ID")
		return
	}

	if err := s.postService.DeletePost(r.Context(), userOID, postOID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, nil, currentUserID(r), "post.delete", "post", postOID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "post deleted successfully",
	})
}

func (s *Server) deleteScheduledPost(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	postOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid post ID")
		return
	}

	if err := s.postService.DeleteScheduledPost(r.Context(), userOID, postOID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, nil, currentUserID(r), "post.delete_scheduled", "post", postOID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "scheduled post deleted successfully",
	})
}

func (s *Server) deletePublishedPost(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	postOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid post ID")
		return
	}

	if err := s.postService.DeletePublishedPost(r.Context(), userOID, postOID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, nil, currentUserID(r), "post.delete_published", "post", postOID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "published post and remote platform records deleted successfully",
	})
}

func (s *Server) bulkDeletePosts(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input models.BulkIDsInput
	if !decode(w, r, 0, &input) {
		return
	}

	count, err := s.postService.BulkDeletePosts(r.Context(), userOID, input.IDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"deleted": count,
	})
}

func (s *Server) bulkDeleteScheduledPosts(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input models.BulkIDsInput
	if !decode(w, r, 0, &input) {
		return
	}

	deleted, failed, err := s.postService.BulkDeleteScheduledPosts(r.Context(), userOID, input.IDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"deleted": deleted,
		"failed":  failed,
	})
}

func (s *Server) bulkDeletePublishedPosts(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input models.BulkIDsInput
	if !decode(w, r, 0, &input) {
		return
	}

	deleted, failed, err := s.postService.BulkDeletePublishedPosts(r.Context(), userOID, input.IDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"deleted": deleted,
		"failed":  failed,
	})
}

func (s *Server) bulkPauseRecurringPosts(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input models.BulkIDsInput
	if !decode(w, r, 0, &input) {
		return
	}

	paused, failed, err := s.postService.BulkPauseRecurringPosts(r.Context(), userOID, input.IDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"paused":  paused,
		"failed":  failed,
	})
}

func (s *Server) bulkResumeRecurringPosts(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input models.BulkIDsInput
	if !decode(w, r, 0, &input) {
		return
	}

	resumed, published, failed, err := s.postService.BulkResumeRecurringPosts(r.Context(), userOID, input.IDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"resumed":   resumed,
		"published": published,
		"failed":    failed,
	})
}

func (s *Server) submitPostForApproval(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	postOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid post ID")
		return
	}

	post, err := s.postService.SubmitForApproval(r.Context(), userOID, postOID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	reviewURL := fmt.Sprintf("/v1/posts/review/%s", post.ApprovalToken)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":       true,
		"post":          post,
		"approvalToken": post.ApprovalToken,
		"reviewUrl":     reviewURL,
	})
}

func (s *Server) approvePost(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	postOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid post ID")
		return
	}

	var req struct {
		Comment string `json:"comment"`
	}
	if r.Body != nil && r.ContentLength > 0 {
		_ = decode(w, r, 0, &req)
	}

	post, err := s.postService.ApprovePost(r.Context(), userOID, postOID, req.Comment)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, post.BrandID, currentUserID(r), "post.approve", "post", postOID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"post":    post,
	})
}

func (s *Server) rejectPost(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	postOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid post ID")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if r.Body != nil && r.ContentLength > 0 {
		_ = decode(w, r, 0, &req)
	}

	post, err := s.postService.RejectPost(r.Context(), userOID, postOID, req.Reason)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, post.BrandID, currentUserID(r), "post.reject", "post", postOID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"post":    post,
	})
}

func (s *Server) publicReview(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "approval token is required")
		return
	}

	post, err := s.postService.FindByApprovalToken(r.Context(), token)
	if err != nil || post == nil {
		writeError(w, http.StatusNotFound, "review link invalid or post not found")
		return
	}

	writeJSON(w, http.StatusOK, post)
}

func (s *Server) publicReviewAction(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "approval token is required")
		return
	}

	var req struct {
		Action       string `json:"action"` // "approve" or "reject"
		ReviewerName string `json:"reviewerName"`
		Feedback     string `json:"feedback"`
	}
	if !decode(w, r, 0, &req) {
		return
	}

	post, err := s.postService.PublicReviewAction(r.Context(), token, req.Action, req.ReviewerName, req.Feedback)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"post":    post,
	})
}
