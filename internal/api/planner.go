package api

import (
	"net/http"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Server) getBestTimes(w http.ResponseWriter, r *http.Request) {
	platform := r.URL.Query().Get("platform")
	if platform == "" {
		platform = "instagram"
	}

	brandOID, _ := currentBrandOID(r)
	if brandOID.IsZero() {
		// Try from query
		if b := r.URL.Query().Get("brandId"); b != "" {
			if oid, err := bson.ObjectIDFromHex(b); err == nil {
				brandOID = oid
			}
		}
	}

	bestTimes, err := s.bestTimesService.GetBestTimes(r.Context(), brandOID, platform)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to calculate best posting times: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, bestTimes)
}
