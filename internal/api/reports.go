package api

import (
	"fmt"
	"net/http"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
)

func (s *Server) generateReport(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req models.GenerateReportRequest
	if !decode(w, r, 0, &req) {
		return
	}

	if req.BrandID == "" {
		if bOID, ok := currentBrandOID(r); ok {
			req.BrandID = bOID.Hex()
		}
	}

	report, err := s.reportService.GenerateReport(r.Context(), userOID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, &report.BrandID, currentUserID(r), "report.generate", "report", report.ID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, report)
}

func (s *Server) listReports(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	reports, err := s.reportService.ListReports(r.Context(), brandOID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list reports: "+err.Error())
		return
	}
	if reports == nil {
		reports = []models.Report{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  reports,
		"count": len(reports),
	})
}

func (s *Server) downloadReport(w http.ResponseWriter, r *http.Request) {
	reportOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid report ID")
		return
	}

	report, err := s.reportService.GetReport(r.Context(), reportOID)
	if err != nil || report == nil {
		writeError(w, http.StatusNotFound, "report not found")
		return
	}

	if report.FileURL != "" {
		http.Redirect(w, r, report.FileURL, http.StatusTemporaryRedirect)
		return
	}

	// Generate downloadable mock summary PDF / text
	content := fmt.Sprintf("%%PDF-1.4\n1 0 obj\n<< /Title (%s) /Author (xSocial) >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%%%EOF", report.Title)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"report-%s.pdf\"", report.ID.Hex()))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(content))
}
