package api

import (
	"net/http"

	"xsocial-backend/internal/models"
)

func (s *Server) createBrand(w http.ResponseWriter, r *http.Request) {
	userOID, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	var req models.CreateBrandRequest
	if !decode(w, r, 0, &req) {
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	brand, err := s.brandService.CreateBrand(r.Context(), userOID, req)
	if err != nil {
		writeApiError(w, err)
		return
	}

	s.logAudit(r, &userOID, &brand.ID, "brand.create", "brand", brand.ID.Hex())
	writeJSON(w, http.StatusCreated, brand)
}

func (s *Server) listBrands(w http.ResponseWriter, r *http.Request) {
	userOID, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	brands, err := s.brandService.ListBrands(r.Context(), userOID)
	if err != nil {
		writeApiError(w, err)
		return
	}
	if brands == nil {
		brands = []models.Brand{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    brands,
		"count":   len(brands),
	})
}

func (s *Server) getBrand(w http.ResponseWriter, r *http.Request) {
	userOID, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	brandOID, ok := s.parsePathOID(w, r, "id", "brand")
	if !ok {
		return
	}

	brand, err := s.brandService.GetBrand(r.Context(), brandOID, userOID)
	if err != nil {
		writeApiError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, brand)
}

func (s *Server) updateBrand(w http.ResponseWriter, r *http.Request) {
	userOID, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	brandOID, ok := s.parsePathOID(w, r, "id", "brand")
	if !ok {
		return
	}

	var req models.UpdateBrandRequest
	if !decode(w, r, 0, &req) {
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	brand, err := s.brandService.UpdateBrand(r.Context(), brandOID, userOID, req)
	if err != nil {
		writeApiError(w, err)
		return
	}

	s.logAudit(r, &userOID, &brandOID, "brand.update", "brand", brandOID.Hex())
	writeJSON(w, http.StatusOK, brand)
}

func (s *Server) deleteBrand(w http.ResponseWriter, r *http.Request) {
	userOID, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	brandOID, ok := s.parsePathOID(w, r, "id", "brand")
	if !ok {
		return
	}

	if err := s.brandService.DeleteBrand(r.Context(), brandOID, userOID); err != nil {
		writeApiError(w, err)
		return
	}

	s.logAudit(r, &userOID, &brandOID, "brand.delete", "brand", brandOID.Hex())
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "brand deleted successfully",
	})
}

func (s *Server) addBrandMember(w http.ResponseWriter, r *http.Request) {
	userOID, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	brandOID, ok := s.parsePathOID(w, r, "id", "brand")
	if !ok {
		return
	}

	var req models.AddBrandMemberRequest
	if !decode(w, r, 0, &req) {
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	member, err := s.brandService.AddMember(r.Context(), brandOID, userOID, req)
	if err != nil {
		writeApiError(w, err)
		return
	}

	s.logAudit(r, &userOID, &brandOID, "brand.member_add", "brand_member", member.ID.Hex())
	writeJSON(w, http.StatusCreated, member)
}

func (s *Server) listBrandMembers(w http.ResponseWriter, r *http.Request) {
	userOID, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	brandOID, ok := s.parsePathOID(w, r, "id", "brand")
	if !ok {
		return
	}

	members, err := s.brandService.ListMembers(r.Context(), brandOID, userOID)
	if err != nil {
		writeApiError(w, err)
		return
	}
	if members == nil {
		members = []models.BrandMember{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    members,
		"count":   len(members),
	})
}

func (s *Server) removeBrandMember(w http.ResponseWriter, r *http.Request) {
	userOID, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	brandOID, ok := s.parsePathOID(w, r, "id", "brand")
	if !ok {
		return
	}

	targetOID, ok := s.parsePathOID(w, r, "userId", "member user")
	if !ok {
		return
	}

	if err := s.brandService.RemoveMember(r.Context(), brandOID, targetOID, userOID); err != nil {
		writeApiError(w, err)
		return
	}

	s.logAudit(r, &userOID, &brandOID, "brand.member_remove", "brand_member", targetOID.Hex())
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "member removed successfully",
	})
}
