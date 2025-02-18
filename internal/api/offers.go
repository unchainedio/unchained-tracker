package api

import (
    "encoding/json"
    "net/http"
    "strings"
    "unchained-tracker/internal/db"
)

func (s *Server) HandleOffers(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case "GET":
        s.getOffers(w, r)
    case "POST":
        s.createOffer(w, r)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

func (s *Server) getOffers(w http.ResponseWriter, r *http.Request) {
    offers, err := s.db.GetOffers()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(offers)
}

func (s *Server) createOffer(w http.ResponseWriter, r *http.Request) {
    var offer db.Offer
    if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Validate required fields
    if offer.Name == "" || offer.Network == "" || offer.OfferURL == "" {
        http.Error(w, "name, network, and offer_url are required", http.StatusBadRequest)
        return
    }

    if err := s.db.SaveOffer(&offer); err != nil {
        if strings.Contains(err.Error(), "already exists") {
            http.Error(w, err.Error(), http.StatusConflict)
            return
        }
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(offer)
} 