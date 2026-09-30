package api

import (
	"net/http"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
)

func (s *Server) createAutolist(w http.ResponseWriter, r *http.Request) {
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

	var req models.CreateAutolistRequest
	if !decode(w, r, 0, &req) {
		return
	}

	list, err := s.autolistService.CreateAutolist(r.Context(), brandOID, req)
	if err != nil {
		writeApiError(w, err)
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, &brandOID, currentUserID(r), "autolist.create", "autolist", list.ID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, list)
}

func (s *Server) listAutolists(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	lists, err := s.autolistService.ListAutolists(r.Context(), brandOID)
	if err != nil {
		writeApiError(w, err)
		return
	}
	if lists == nil {
		lists = []models.Autolist{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  lists,
		"count": len(lists),
	})
}

func (s *Server) addAutolistPost(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	autolistOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid autolist ID")
		return
	}

	var req models.AddAutolistPostRequest
	if !decode(w, r, 0, &req) {
		return
	}

	list, err := s.autolistService.AddPostToAutolist(r.Context(), autolistOID, brandOID, req)
	if err != nil {
		writeApiError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, list)
}

func (s *Server) deleteAutolist(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	autolistOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid autolist ID")
		return
	}

	if err := s.autolistService.DeleteAutolist(r.Context(), autolistOID, brandOID); err != nil {
		writeApiError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "autolist deleted successfully",
	})
}

func (s *Server) createRSSFeed(w http.ResponseWriter, r *http.Request) {
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

	var req models.CreateRSSFeedRequest
	if !decode(w, r, 0, &req) {
		return
	}

	feed, err := s.autolistService.CreateRSSFeed(r.Context(), brandOID, req)
	if err != nil {
		writeApiError(w, err)
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, &brandOID, currentUserID(r), "rss_feed.create", "rss_feed", feed.ID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, feed)
}

func (s *Server) listRSSFeeds(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	feeds, err := s.autolistService.ListRSSFeeds(r.Context(), brandOID)
	if err != nil {
		writeApiError(w, err)
		return
	}
	if feeds == nil {
		feeds = []models.RSSFeed{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  feeds,
		"count": len(feeds),
	})
}

func (s *Server) deleteRSSFeed(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	feedOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid RSS feed ID")
		return
	}

	if err := s.autolistService.DeleteRSSFeed(r.Context(), feedOID, brandOID); err != nil {
		writeApiError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "RSS feed deleted successfully",
	})
}
