package api

import (
    "encoding/json"
    "net/http"
    "time"
    "github.com/google/uuid"
    "unchained-tracker/internal/db"
    "log"
    "crypto/rand"
    "fmt"
)

func generateCampaignToken() string {
    // Generate 10-digit random number
    random := make([]byte, 5)
    rand.Read(random)
    return fmt.Sprintf("%010d", random)
}

type CampaignRequest struct {
    Name          string `json:"name"`
    LandingPage   string `json:"landing_page"`
    TrafficSource string `json:"traffic_source"`
    OfferURL      string `json:"offer_url"`
}

type CampaignResponse struct {
    ID            int64     `json:"id"`
    Name          string    `json:"name"`
    CampaignID    string    `json:"campaign_id"`
    CampaignToken string    `json:"campaign_token"`
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
        log.Printf("Received campaign creation request")
        s.createCampaign(w, r)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

func (s *Server) createCampaign(w http.ResponseWriter, r *http.Request) {
    var req CampaignRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        log.Printf("Error decoding campaign: %v", err)
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
        OfferURL:      req.OfferURL,
        LandingPage:   req.LandingPage,
        TrafficSource: req.TrafficSource,
        CreatedAt:     time.Now(),
    }

    log.Printf("Creating campaign: %+v", campaign)

    // Generate campaign token if not provided
    if campaign.CampaignToken == "" {
        campaign.CampaignToken = generateCampaignToken()
    }

    if err := s.db.SaveCampaign(campaign); err != nil {
        log.Printf("Error saving campaign: %v", err)
        http.Error(w, "Error creating campaign", http.StatusInternalServerError)
        return
    }

    log.Printf("Campaign created successfully: %+v", campaign)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status":      "success",
        "campaign_id": campaignID,
        "campaign_token": campaign.CampaignToken,
    })
}

func (s *Server) listCampaigns(w http.ResponseWriter, r *http.Request) {
    log.Printf("Fetching campaign list")
    stats, err := s.db.GetCampaignStats()
    if err != nil {
        log.Printf("Error getting campaign stats: %+v", err)
        http.Error(w, "Error fetching campaigns", http.StatusInternalServerError)
        return
    }

    if len(stats) == 0 {
        log.Printf("No campaigns found")
        // Return empty array instead of null
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode([]CampaignResponse{})
        return
    }

    // Convert to response format
    var response []CampaignResponse
    for _, stat := range stats {
        log.Printf("Processing campaign: %+v", stat)
        resp := CampaignResponse{
            ID:            stat.ID,
            Name:          stat.Name,
            CampaignID:    stat.CampaignID,
            CampaignToken: stat.CampaignToken,
            LandingPage:   stat.LandingPage,
            TrafficSource: stat.TrafficSource,
            CreatedAt:     stat.CreatedAt,
        }
        resp.Stats.Visits = stat.Visits
        resp.Stats.Conversions = stat.Conversions
        resp.Stats.Revenue = stat.Revenue
        response = append(response, resp)
    }

    log.Printf("Found %d campaigns", len(response))
    log.Printf("Response data: %+v", response)
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