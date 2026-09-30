package api

import (
	"net/http"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Server) resolveBrandForRequest(r *http.Request) (bson.ObjectID, error) {
	if bID, ok := currentBrandOID(r); ok {
		return bID, nil
	}
	if uID, ok := currentUserOID(r); ok {
		brands, err := s.brandRepo.FindBrandsByUserID(r.Context(), uID)
		if err == nil && len(brands) > 0 {
			return brands[0].ID, nil
		}
	}
	return bson.NilObjectID, http.ErrNoCookie
}

func (s *Server) listInboxConversations(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	status := r.URL.Query().Get("status")
	platform := r.URL.Query().Get("platform")
	pageNum, limitNum := page(r)

	convs, total, err := s.inboxService.ListConversations(r.Context(), brandOID, userOID, status, platform, int64(pageNum), int64(limitNum))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list conversations: "+err.Error())
		return
	}
	if convs == nil {
		convs = []any{}
		_ = convs
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  convs,
		"page":  pageNum,
		"limit": limitNum,
		"total": total,
	})
}

func (s *Server) getInboxConversation(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	convOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conversation ID")
		return
	}

	conv, messages, err := s.inboxService.GetConversation(r.Context(), convOID, brandOID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"conversation": conv,
		"messages":     messages,
	})
}

func (s *Server) replyInboxMessage(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	convOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conversation ID")
		return
	}

	var req struct {
		Text     string `json:"text"`
		MediaURL string `json:"mediaUrl"`
	}
	if !decode(w, r, 0, &req) {
		return
	}

	msg, err := s.inboxService.ReplyToConversation(r.Context(), convOID, brandOID, userOID, req.Text, req.MediaURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, &brandOID, currentUserID(r), "inbox.reply", "inbox_conversation", convOID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, msg)
}

func (s *Server) updateInboxStatus(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	convOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conversation ID")
		return
	}

	var req struct {
		Status     string  `json:"status"` // "open", "resolved", "archived"
		AssignedTo *string `json:"assignedTo"`
	}
	if !decode(w, r, 0, &req) {
		return
	}

	var assignedOID *bson.ObjectID
	if req.AssignedTo != nil && *req.AssignedTo != "" {
		if oid, err := bson.ObjectIDFromHex(*req.AssignedTo); err == nil {
			assignedOID = &oid
		}
	}

	if err := s.inboxService.UpdateConversationStatus(r.Context(), convOID, brandOID, req.Status, assignedOID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "conversation status updated",
	})
}

func (s *Server) listCannedReplies(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	replies, err := s.inboxService.ListCannedReplies(r.Context(), brandOID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list canned replies")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  replies,
		"count": len(replies),
	})
}

func (s *Server) createCannedReply(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	var req struct {
		Shortcut string   `json:"shortcut"`
		Title    string   `json:"title"`
		Text     string   `json:"text"`
		Tags     []string `json:"tags"`
	}
	if !decode(w, r, 0, &req) {
		return
	}

	canned, err := s.inboxService.CreateCannedReply(r.Context(), brandOID, req.Shortcut, req.Title, req.Text, req.Tags)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, canned)
}

func (s *Server) deleteCannedReply(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	idOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid canned reply ID")
		return
	}

	if err := s.inboxService.DeleteCannedReply(r.Context(), idOID, brandOID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "canned reply deleted successfully",
	})
}
