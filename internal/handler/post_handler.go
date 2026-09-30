package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/service"
	"xsocial-backend/internal/utils"
)

type PostHandler struct {
	postService *service.PostService
}

func NewPostHandler(postService *service.PostService) *PostHandler {
	return &PostHandler{
		postService: postService,
	}
}

func (h *PostHandler) CreatePost(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	var input models.CreatePostInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.Error(utils.BadRequest("Validation failed", err.Error()))
		return
	}

	post, err := h.postService.CreatePost(c.Request.Context(), userOID, input)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusCreated, "Post created", post)
}

func (h *PostHandler) ListPosts(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	page, limit, skip := utils.ParsePagination(c.Query("page"), c.Query("limit"))

	queryParams := map[string]string{
		"status":     c.Query("status"),
		"postType":   c.Query("postType"),
		"search":     c.Query("search"),
		"accountIds": c.Query("accountIds"),
	}

	sortBy := c.Query("sortBy")
	if sortBy == "" {
		sortBy = "createdAt"
	}
	order := -1
	if c.Query("order") == "asc" {
		order = 1
	}
	sort := bson.D{{Key: sortBy, Value: order}}

	items, total, err := h.postService.ListPosts(c.Request.Context(), userOID, queryParams, skip, limit, sort)
	if err != nil {
		c.Error(err)
		return
	}

	meta := utils.PaginationMeta{
		Page:  page,
		Limit: limit,
		Total: total,
	}

	utils.SendSuccess(c, http.StatusOK, "Posts fetched", items, meta)
}

func (h *PostHandler) GetPost(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	postOID, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.Error(utils.BadRequest("Invalid post ID"))
		return
	}

	post, err := h.postService.GetPost(c.Request.Context(), userOID, postOID)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post fetched", post)
}

func (h *PostHandler) UpdatePost(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	postOID, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.Error(utils.BadRequest("Invalid post ID"))
		return
	}

	var input models.UpdatePostInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.Error(utils.BadRequest("Validation failed", err.Error()))
		return
	}

	updated, err := h.postService.UpdatePost(c.Request.Context(), userOID, postOID, input)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post updated", updated)
}

func (h *PostHandler) DeletePost(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	postOID, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.Error(utils.BadRequest("Invalid post ID"))
		return
	}

	if err := h.postService.DeletePost(c.Request.Context(), userOID, postOID); err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post deleted", nil)
}

func (h *PostHandler) DeleteScheduledPost(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	postOID, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.Error(utils.BadRequest("Invalid post ID"))
		return
	}

	if err := h.postService.DeleteScheduledPost(c.Request.Context(), userOID, postOID); err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Scheduled post deleted", nil)
}

func (h *PostHandler) DeletePublishedPost(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	postOID, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.Error(utils.BadRequest("Invalid post ID"))
		return
	}

	if err := h.postService.DeletePublishedPost(c.Request.Context(), userOID, postOID); err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Published post deleted", nil)
}

func (h *PostHandler) BulkDeletePosts(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	var input models.BulkIDsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.Error(utils.BadRequest("Validation failed", err.Error()))
		return
	}

	deletedCount, err := h.postService.BulkDeletePosts(c.Request.Context(), userOID, input.IDs)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Posts deleted", gin.H{"deletedCount": deletedCount})
}

func (h *PostHandler) BulkDeleteScheduledPosts(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	var input models.BulkIDsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.Error(utils.BadRequest("Validation failed", err.Error()))
		return
	}

	deleted, skipped, err := h.postService.BulkDeleteScheduledPosts(c.Request.Context(), userOID, input.IDs)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Scheduled posts deleted", gin.H{
		"deletedCount": deleted,
		"skippedCount": skipped,
	})
}

func (h *PostHandler) BulkDeletePublishedPosts(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	var input models.BulkIDsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.Error(utils.BadRequest("Validation failed", err.Error()))
		return
	}

	deleted, skipped, err := h.postService.BulkDeletePublishedPosts(c.Request.Context(), userOID, input.IDs)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Published posts deleted", gin.H{
		"deletedCount": deleted,
		"skippedCount": skipped,
	})
}

func (h *PostHandler) PublishPost(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	postOID, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.Error(utils.BadRequest("Invalid post ID"))
		return
	}

	var input models.PublishPostInput
	_ = c.ShouldBindJSON(&input)

	post, err := h.postService.PublishPost(c.Request.Context(), userOID, postOID, input.Platforms, input.AccountIds, input.Recurrence)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post publish attempted", post)
}

func (h *PostHandler) SchedulePost(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	postOID, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.Error(utils.BadRequest("Invalid post ID"))
		return
	}

	var input models.SchedulePostInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.Error(utils.BadRequest("Validation failed", err.Error()))
		return
	}

	post, err := h.postService.SchedulePost(c.Request.Context(), userOID, postOID, input)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post scheduled", post)
}

func (h *PostHandler) PauseRecurringPost(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	postOID, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.Error(utils.BadRequest("Invalid post ID"))
		return
	}

	post, err := h.postService.PauseRecurringPost(c.Request.Context(), userOID, postOID)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Recurring schedule paused", post)
}

func (h *PostHandler) ResumeRecurringPost(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	postOID, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.Error(utils.BadRequest("Invalid post ID"))
		return
	}

	post, err := h.postService.ResumeRecurringPost(c.Request.Context(), userOID, postOID)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Recurring schedule resumed", post)
}

func (h *PostHandler) BulkPauseRecurringPosts(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	var input models.BulkIDsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.Error(utils.BadRequest("Validation failed", err.Error()))
		return
	}

	pausedCount, skippedCount, err := h.postService.BulkPauseRecurringPosts(c.Request.Context(), userOID, input.IDs)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Recurring schedules paused", gin.H{
		"pausedCount":  pausedCount,
		"skippedCount": skippedCount,
	})
}

func (h *PostHandler) BulkResumeRecurringPosts(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	var input models.BulkIDsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.Error(utils.BadRequest("Validation failed", err.Error()))
		return
	}

	resumedCount, skippedCount, failedCount, err := h.postService.BulkResumeRecurringPosts(c.Request.Context(), userOID, input.IDs)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Recurring schedules resumed", gin.H{
		"resumedCount": resumedCount,
		"skippedCount": skippedCount,
		"failedCount":  failedCount,
	})
}

func (h *PostHandler) SubmitForApproval(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.Error(utils.BadRequest("Invalid post ID"))
		return
	}

	token := utils.GenerateRandomToken(16)
	updated, err := h.postService.UpdatePost(c.Request.Context(), id, userOID, models.UpdatePostRequest{
		ReviewStatus:  "pending_approval",
		ApprovalToken: token,
	})
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post submitted for approval", gin.H{
		"post":          updated,
		"approvalToken": token,
		"reviewUrl":     "/api/v1/posts/review/" + token,
	})
}

func (h *PostHandler) ApprovePost(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.Error(utils.BadRequest("Invalid post ID"))
		return
	}

	updated, err := h.postService.UpdatePost(c.Request.Context(), id, userOID, models.UpdatePostRequest{
		ReviewStatus: "approved",
		Status:       models.PostStatusScheduled,
	})
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post approved and scheduled for publication", updated)
}

func (h *PostHandler) RejectPost(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.Error(utils.BadRequest("Invalid post ID"))
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)

	updated, err := h.postService.UpdatePost(c.Request.Context(), id, userOID, models.UpdatePostRequest{
		ReviewStatus: "rejected",
		Status:       models.PostStatusDraft,
	})
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post returned to draft with rejection feedback", updated)
}

func (h *PostHandler) PublicReview(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Review token is required"})
		return
	}

	// Find post by approvalToken
	post, err := h.postService.FindByApprovalToken(c.Request.Context(), token)
	if err != nil || post == nil {
		c.JSON(http.StatusNotFound, utils.APIResponse{Success: false, Message: "Review link is invalid or expired"})
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Data:    post,
	})
}

func (h *PostHandler) PublicReviewAction(c *gin.Context) {
	token := c.Param("token")
	var req struct {
		Action string `json:"action" binding:"required"` // "approve" or "reject"
		Notes  string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Action is required"})
		return
	}

	post, err := h.postService.FindByApprovalToken(c.Request.Context(), token)
	if err != nil || post == nil {
		c.JSON(http.StatusNotFound, utils.APIResponse{Success: false, Message: "Review link is invalid or expired"})
		return
	}

	reviewStatus := "approved"
	postStatus := models.PostStatusScheduled
	if req.Action == "reject" {
		reviewStatus = "rejected"
		postStatus = models.PostStatusDraft
	}

	updated, err := h.postService.UpdatePost(c.Request.Context(), post.ID, post.UserID, models.UpdatePostRequest{
		ReviewStatus: reviewStatus,
		Status:       postStatus,
	})
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Message: "Review action recorded successfully",
		Data:    updated,
	})
}
