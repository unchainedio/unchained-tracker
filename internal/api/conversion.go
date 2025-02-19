package api

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "time"
    "unchained-tracker/internal/db"
)

type ConversionRequest struct {
    VisitorID  string  `json:"visitor_id"`
    ClickID    string  `json:"click_id"`
    CampaignID string  `json:"campaign_id"`
    Amount     float64 `json:"amount"`
}

type FacebookEvent struct {
    Data []struct {
        EventName string `json:"event_name"`
        EventTime int64  `json:"event_time"`
        UserData  struct {
            ClientIPAddress string `json:"client_ip_address"`
            ClientUserAgent string `json:"client_user_agent"`
        } `json:"user_data"`
        CustomData struct {
            Value      float64 `json:"value"`
            Currency   string  `json:"currency"`
            CampaignID string  `json:"campaign_id"`
            ClickID    string  `json:"click_id"`
        } `json:"custom_data"`
    } `json:"data"`
    AccessToken string `json:"access_token"`
}

// Facebook cookie helper functions
func getFBCookie(r *http.Request) string {
    if cookie, err := r.Cookie("_fbc"); err == nil {
        return cookie.Value
    }
    return ""
}

func getFBPCookie(r *http.Request) string {
    if cookie, err := r.Cookie("_fbp"); err == nil {
        return cookie.Value
    }
    return ""
}

func (s *Server) HandleConversion(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var req ConversionRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    log.Printf("Received conversion request: %+v", req)

    // Validate required fields
    if req.VisitorID == "" {
        http.Error(w, "Missing visitor_id", http.StatusBadRequest)
        return
    }

    conversion := &db.Conversion{
        VisitorID:  req.VisitorID,
        ClickID:    req.ClickID,
        CampaignID: req.CampaignID,
        Amount:     req.Amount,
        Status:     "completed",
        CreatedAt:  time.Now(),
    }

    if err := s.db.SaveConversion(conversion); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Optional: Send to Facebook Conversion API
    if s.config.FacebookEnabled {
        go s.sendToFacebook(conversion, r)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status":        "success",
        "conversion_id": conversion.ID,
        "amount":        conversion.Amount,
        "created_at":    conversion.CreatedAt,
        "visitor_id":    conversion.VisitorID,
        "campaign_id":   conversion.CampaignID,
    })
}

func (s *Server) sendToFacebook(conversion *db.Conversion, r *http.Request) error {
    if !s.config.FacebookEnabled {
        return nil
    }

    // Get visit info for the conversion
    visit, err := s.db.GetVisitByVisitorID(conversion.VisitorID)
    if err != nil {
        return fmt.Errorf("error getting visit info: %v", err)
    }

    // Prepare Facebook event data
    event := map[string]interface{}{
        "data": []map[string]interface{}{
            {
                "event_name": "Purchase",
                "event_time": time.Now().Unix(),
                "user_data": map[string]interface{}{
                    "client_ip_address": visit.IPAddress,
                    "client_user_agent": visit.UserAgent,
                    "fbc":              getFBCookie(r),
                    "fbp":              getFBPCookie(r),
                },
                "custom_data": map[string]interface{}{
                    "value": conversion.Amount,
                    "currency": "USD",
                    "campaign_id": conversion.CampaignID,
                    "click_id": conversion.ClickID,
                },
            },
        },
        "access_token": s.config.FacebookToken,
        "pixel_id":     s.config.FacebookPixelID,
    }

    // Send to Facebook
    jsonData, err := json.Marshal(event)
    if err != nil {
        return fmt.Errorf("error marshaling event: %v", err)
    }

    url := fmt.Sprintf("https://graph.facebook.com/v13.0/%s/events", s.config.FacebookPixelID)
    resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("error sending to Facebook: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := ioutil.ReadAll(resp.Body)
        return fmt.Errorf("Facebook API error (status %d): %s", resp.StatusCode, string(body))
    }

    return nil
} 