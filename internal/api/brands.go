package api

import (
	"net/http"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
)

func (s *Server) createBrand(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req models.CreateBrandRequest
	if !decode(w, r, 0, &req) {
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "brand name is required")
		return
	}

	brand, err := s.brandService.CreateBrand(r.Context(), userOID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, &brand.ID, currentUserID(r), "brand.create", "brand", brand.ID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, brand)
}

func (s *Server) listBrands(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	brands, err := s.brandService.ListBrands(r.Context(), userOID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list brands")
		return
	}
	if brands == nil {
		brands = []models.Brand{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  brands,
		"count": len(brands),
	})
}

func (s *Server) getBrand(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	brandOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid brand ID")
		return
	}

	brand, err := s.brandService.GetBrand(r.Context(), brandOID, userOID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, brand)
}

func (s *Server) updateBrand(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	brandOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid brand ID")
		return
	}

	var req models.UpdateBrandRequest
	if !decode(w, r, 0, &req) {
		return
	}

	brand, err := s.brandService.UpdateBrand(r.Context(), brandOID, userOID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, &brandOID, currentUserID(r), "brand.update", "brand", brandOID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, brand)
}

func (s *Server) deleteBrand(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	brandOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid brand ID")
		return
	}

	if err := s.brandService.DeleteBrand(r.Context(), brandOID, userOID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, &brandOID, currentUserID(r), "brand.delete", "brand", brandOID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "brand deleted successfully",
	})
}

func (s *Server) addBrandMember(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	brandOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid brand ID")
		return
	}

	var req models.AddBrandMemberRequest
	if !decode(w, r, 0, &req) {
		return
	}

	member, err := s.brandService.AddMember(r.Context(), brandOID, userOID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, &brandOID, currentUserID(r), "brand.member_add", "brand_member", member.ID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, member)
}

func (s *Server) listBrandMembers(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	brandOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid brand ID")
		return
	}

	members, err := s.brandService.ListMembers(r.Context(), brandOID, userOID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if members == nil {
		members = []models.BrandMember{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  members,
		"count": len(members),
	})
}

func (s *Server) removeBrandMember(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	brandOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid brand ID")
		return
	}

	targetOID, err := bson.ObjectIDFromHex(r.PathValue("userId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid member user ID")
		return
	}

	if err := s.brandService.RemoveMember(r.Context(), brandOID, targetOID, userOID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, &brandOID, currentUserID(r), "brand.member_remove", "brand_member", targetOID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "member removed successfully",
	})
}
