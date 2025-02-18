package api

import (
    "encoding/json"
    "net/http"
    "strconv"
    "strings"
    "unchained-tracker/internal/db"
)

func (s *Server) HandleLandingPages(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case "GET":
        s.getLandingPages(w, r)
    case "POST":
        s.createLandingPage(w, r)
    case "PUT":
        s.updateLandingPage(w, r)
    case "DELETE":
        s.deleteLandingPage(w, r)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

func (s *Server) getLandingPages(w http.ResponseWriter, r *http.Request) {
    pages, err := s.db.GetLandingPages()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(pages)
}

func (s *Server) createLandingPage(w http.ResponseWriter, r *http.Request) {
    var page db.LandingPage
    if err := json.NewDecoder(r.Body).Decode(&page); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    if err := s.db.SaveLandingPage(&page); err != nil {
        if strings.Contains(err.Error(), "unique_url") {
            http.Error(w, "URL already exists", http.StatusConflict)
            return
        }
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(page)
}

func (s *Server) updateLandingPage(w http.ResponseWriter, r *http.Request) {
    var page db.LandingPage
    if err := json.NewDecoder(r.Body).Decode(&page); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    if err := s.db.UpdateLandingPage(&page); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(page)
}

func (s *Server) deleteLandingPage(w http.ResponseWriter, r *http.Request) {
    idStr := r.URL.Query().Get("id")
    id, err := strconv.ParseInt(idStr, 10, 64)
    if err != nil {
        http.Error(w, "Invalid ID", http.StatusBadRequest)
        return
    }

    if err := s.db.DeleteLandingPage(id); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
} 