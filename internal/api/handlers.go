package api

import (
    "encoding/json"
    "net/http"
    "time"
    "github.com/google/uuid"
    "unchained-tracker/internal/db"
    "database/sql"
    "log"
)

type VisitRequest struct {
    ClickID          string `json:"click_id"`
    CampaignID       string `json:"campaign_id"`
    UserAgent        string `json:"user_agent"`
    Browser          string `json:"browser"`
    BrowserVersion   string `json:"browser_version"`
    OS               string `json:"os"`
    DeviceType       string `json:"device_type"`
    ScreenResolution string `json:"screen_resolution"`
    ViewportSize     string `json:"viewport_size"`
    Language         string `json:"language"`
    Timezone         string `json:"timezone"`
    LandingPage      string `json:"landing_page"`
    Referrer         string `json:"referrer"`
    UTMSource        string `json:"utm_source"`
    UTMMedium        string `json:"utm_medium"`
    UTMCampaign      string `json:"utm_campaign"`
    UTMContent       string `json:"utm_content"`
    UTMTerm          string `json:"utm_term"`
}

func (s *Server) HandleVisit(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var req VisitRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Check if visit already exists for this click_id
    var existingVisitorID string
    log.Printf("Checking for existing visit with click_id: %s", req.ClickID)
    err := s.db.QueryRow("SELECT visitor_id FROM visit WHERE click_id = ?", req.ClickID).Scan(&existingVisitorID)
    if err != sql.ErrNoRows {
        if err == nil {
            log.Printf("Found existing visit: visitor_id=%s", existingVisitorID)
            // Visit exists, return the existing visitor_id
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(map[string]interface{}{
                "status": "success",
                "visitor_id": existingVisitorID,
                "message": "Visit already tracked",
            })
            return
        }
        // Some other error occurred
        log.Printf("Error checking for existing visit: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Generate new visitor_id
    visitorID := uuid.New().String()
    
    log.Printf("Creating new visit: click_id=%s visitor_id=%s", req.ClickID, visitorID)

    // Get location info
    country, region, city, err := s.geo.GetLocation(r.RemoteAddr)
    if err != nil {
        log.Printf("Error getting location: %v", err)
        // Don't fail the request, just log the error
    }

    visit := &db.Visit{
        VisitorID:        visitorID,
        ClickID:          req.ClickID,
        CampaignID:       req.CampaignID,
        IPAddress:        r.RemoteAddr,
        UserAgent:        req.UserAgent,
        Browser:          req.Browser,
        BrowserVersion:   req.BrowserVersion,
        OS:              req.OS,
        DeviceType:       req.DeviceType,
        ScreenResolution: req.ScreenResolution,
        ViewportSize:     req.ViewportSize,
        Language:         req.Language,
        Timezone:         req.Timezone,
        LandingPage:      req.LandingPage,
        Referrer:         req.Referrer,
        UTMSource:        req.UTMSource,
        UTMMedium:        req.UTMMedium,
        UTMCampaign:      req.UTMCampaign,
        UTMContent:       req.UTMContent,
        UTMTerm:          req.UTMTerm,
        Country:          country,
        Region:           region,
        City:             city,
        CreatedAt:        time.Now(),
    }

    if err := s.db.SaveVisit(visit); err != nil {
        log.Printf("Error saving visit: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status":     "success",
        "visitor_id": visitorID,
    })
} 