package api

import (
	"encoding/json"
	"net/http"
	"unchained-tracker/internal/cloudflare"
	"unchained-tracker/internal/db"
)

func (s *Server) HandleTrackingDomains(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		var domain db.TrackingDomain
		if err := json.NewDecoder(r.Body).Decode(&domain); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Create Cloudflare DNS record
		cf := cloudflare.NewClient(s.config.CloudflareToken)
		if err := cf.CreateDNSRecord(domain.CloudflareZoneID, domain.Domain, s.config.ServerIP); err != nil {
			http.Error(w, "Failed to create DNS record", http.StatusInternalServerError)
			return
		}

		// Save domain to database
		if err := s.db.SaveTrackingDomain(&domain); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(domain)

	case "GET":
		domains, err := s.db.GetTrackingDomains()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(domains)
	}
} 