package api

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "strconv"
    "time"
    "unchained-tracker/internal/db"
)

// NetworkPostback represents different network parameter formats
type NetworkPostback struct {
    ClickID    string  `json:"click_id"`
    Amount     float64 `json:"amount"`
    Network    string  `json:"network"`
    Status     string  `json:"status"`
    ExternalID string  `json:"external_id"`
}

func (s *Server) HandleNetworkPostback(w http.ResponseWriter, r *http.Request) {
    log.Printf("Received network postback: %s %s", r.Method, r.URL.String())

    // Parse parameters based on known network formats
    postback := parseNetworkParameters(r)
    
    log.Printf("Parsed postback parameters: %+v", postback)
    
    if postback.ClickID == "" {
        http.Error(w, "Missing click_id parameter", http.StatusBadRequest)
        return
    }

    log.Printf("Looking for visit with click_id: %s", postback.ClickID)

    // Get visit info from click_id
    visit, err := s.db.GetVisitByClickID(postback.ClickID)
    if err != nil {
        log.Printf("Error finding visit: %v", err)
        http.Error(w, fmt.Sprintf("Visit not found for click_id: %s", postback.ClickID), http.StatusNotFound)
        return
    }

    log.Printf("Found visit: %+v", visit)

    // Create conversion
    conversion := &db.Conversion{
        VisitorID:  visit.VisitorID,
        ClickID:    postback.ClickID,
        CampaignID: visit.CampaignID,
        Amount:     postback.Amount,
        Status:     postback.Status,
        CreatedAt:  time.Now(),
    }

    if err := s.db.SaveConversion(conversion); err != nil {
        log.Printf("Error saving conversion: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    log.Printf("Saved conversion: id=%d amount=%.2f", conversion.ID, conversion.Amount)

    // Optional: Send to Facebook
    if s.cfg.FacebookEnabled {
        go s.sendToFacebook(conversion, r)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status":        "success",
        "conversion_id": conversion.ID,
        "amount":        conversion.Amount,
        "click_id":      conversion.ClickID,
        "network":       postback.Network,
    })
}

func parseNetworkParameters(r *http.Request) NetworkPostback {
    // Get all possible parameter names
    clickIDParams := []string{"click_id", "clickid", "click", "id"}
    amountParams := []string{"amount", "payout", "revenue"}
    
    var postback NetworkPostback
    
    // Find click_id
    for _, param := range clickIDParams {
        if val := r.URL.Query().Get(param); val != "" {
            postback.ClickID = val
            break
        }
    }

    // Find amount
    for _, param := range amountParams {
        if val := r.URL.Query().Get(param); val != "" {
            amount, err := strconv.ParseFloat(val, 64)
            if err == nil {
                postback.Amount = amount
                break
            }
        }
    }

    // Set defaults
    if postback.Status == "" {
        postback.Status = "completed"
    }
    
    // Try to identify network
    if r.URL.Query().Get("network") != "" {
        postback.Network = r.URL.Query().Get("network")
    } else {
        postback.Network = "unknown"
    }

    return postback
} 