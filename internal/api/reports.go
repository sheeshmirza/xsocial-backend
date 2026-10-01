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
		if bOID, err := s.resolveBrandForRequest(r); err == nil {
			req.BrandID = bOID.Hex()
		}
	}

	report, err := s.reportService.GenerateReport(r.Context(), userOID, req)
	if err != nil {
		writeApiError(w, err)
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
		writeApiError(w, err)
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

	_, filename, contentType, data, err := s.reportService.BuildReportExportData(r.Context(), reportOID)
	if err != nil {
		writeApiError(w, err)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
