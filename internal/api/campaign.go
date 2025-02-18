package api

import (
    "encoding/json"
    "net/http"
    "time"
    "github.com/google/uuid"
    "unchained-tracker/internal/db"
)

type CampaignRequest struct {
    Name          string `json:"name"`
    LandingPage   string `json:"landing_page"`
    TrafficSource string `json:"traffic_source"`
}

type CampaignResponse struct {
    ID            int64     `json:"id"`
    Name          string    `json:"name"`
    CampaignID    string    `json:"campaign_id"`
    LandingPage   string    `json:"landing_page"`
    TrafficSource string    `json:"traffic_source"`
    CreatedAt     time.Time `json:"created_at"`
    Stats         struct {
        Visits      int64   `json:"visits"`
        Conversions int64   `json:"conversions"`
        Revenue     float64 `json:"revenue"`
    } `json:"stats"`
}

func (s *Server) HandleCampaigns(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        s.listCampaigns(w, r)
    case http.MethodPost:
        s.createCampaign(w, r)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

func (s *Server) createCampaign(w http.ResponseWriter, r *http.Request) {
    var req CampaignRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // Validate required fields
    if req.Name == "" || req.LandingPage == "" || req.TrafficSource == "" {
        http.Error(w, "Missing required fields", http.StatusBadRequest)
        return
    }

    // Generate unique campaign ID
    campaignID := uuid.New().String()

    campaign := &db.Campaign{
        Name:          req.Name,
        CampaignID:    campaignID,
        LandingPage:   req.LandingPage,
        TrafficSource: req.TrafficSource,
        CreatedAt:     time.Now(),
    }

    if err := s.db.SaveCampaign(campaign); err != nil {
        http.Error(w, "Error creating campaign", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status":      "success",
        "campaign_id": campaignID,
    })
}

func (s *Server) listCampaigns(w http.ResponseWriter, r *http.Request) {
    stats, err := s.db.GetCampaignStats()
    if err != nil {
        http.Error(w, "Error fetching campaigns", http.StatusInternalServerError)
        return
    }

    // Convert to response format
    var response []CampaignResponse
    for _, stat := range stats {
        resp := CampaignResponse{
            ID:            stat.ID,
            Name:          stat.Name,
            CampaignID:    stat.CampaignID,
            LandingPage:   stat.LandingPage,
            TrafficSource: stat.TrafficSource,
            CreatedAt:     stat.CreatedAt,
        }
        resp.Stats.Visits = stat.Visits
        resp.Stats.Conversions = stat.Conversions
        resp.Stats.Revenue = stat.Revenue
        response = append(response, resp)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// Add campaign deletion if needed
func (s *Server) deleteCampaign(w http.ResponseWriter, r *http.Request) {
    campaignID := r.URL.Query().Get("id")
    if campaignID == "" {
        http.Error(w, "Missing campaign ID", http.StatusBadRequest)
        return
    }

    if err := s.db.DeleteCampaign(campaignID); err != nil {
        http.Error(w, "Error deleting campaign", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "status": "success",
    })
} 