package api

import (
    "encoding/json"
    "net/http"
    "time"
    "html/template"
    "unchained-tracker/internal/db"
)

type DashboardStats struct {
    TodayVisits      int64                    `json:"today_visits"`
    TotalVisits      int64                    `json:"total_visits"`
    TotalConversions int64                    `json:"total_conversions"`
    RecentVisits     []VisitData              `json:"recent_visits"`
    Revenue          float64                   `json:"revenue"`
    Campaigns        []db.CampaignStats       `json:"campaigns"`
}

type VisitData struct {
    ID        int64     `json:"id"`
    VisitorID string    `json:"visitor_id"`
    CampaignID string   `json:"campaign_id"`
    Device    struct {
        Type     string `json:"type"`
        OS       string `json:"os"`
        Browser  string `json:"browser"`
        Screen   string `json:"screen"`
        Viewport string `json:"viewport"`
    } `json:"device"`
    Location struct {
        IP      string `json:"ip"`
        Country string `json:"country"`
        Region  string `json:"region"`
        City    string `json:"city"`
    } `json:"location"`
    Page struct {
        URL      string `json:"url"`
        Referrer string `json:"referrer"`
    } `json:"page"`
    UTM struct {
        Source   string `json:"source"`
        Medium   string `json:"medium"`
        Campaign string `json:"campaign"`
        Content  string `json:"content"`
        Term     string `json:"term"`
    } `json:"utm"`
    Meta struct {
        Language  string    `json:"language"`
        Timezone  string    `json:"timezone"`
        CreatedAt time.Time `json:"created_at"`
    } `json:"meta"`
    Conversions []ConversionData `json:"conversions"`
}

type ConversionData struct {
    ID        int64     `json:"id"`
    Amount    float64   `json:"amount"`
    Status    string    `json:"status"`
    CreatedAt time.Time `json:"created_at"`
}

func (s *Server) ServeDashboard(w http.ResponseWriter, r *http.Request) {
    tmpl, err := template.ParseFiles("static/dashboard.html")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    tmpl.Execute(w, nil)
}

func (s *Server) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    stats := &DashboardStats{}
    var err error

    // Get today's visits
    today := time.Now().Truncate(24 * time.Hour)
    stats.TodayVisits, err = s.db.GetVisitCountSince(today)
    if err != nil {
        http.Error(w, "Error getting today's visits", http.StatusInternalServerError)
        return
    }

    // Get recent visits with conversions
    recentVisits, err := s.db.GetRecentVisitsWithConversions(50)
    if err != nil {
        http.Error(w, "Error getting recent visits", http.StatusInternalServerError)
        return
    }

    // Transform visits into response format
    for _, v := range recentVisits {
        visitData := VisitData{
            ID:         v.ID,
            VisitorID:  v.VisitorID,
            CampaignID: v.CampaignID,
        }

        // Device info
        visitData.Device.Type = v.DeviceType
        visitData.Device.OS = v.OS
        visitData.Device.Browser = v.Browser + " " + v.BrowserVersion
        visitData.Device.Screen = v.ScreenResolution
        visitData.Device.Viewport = v.ViewportSize

        // Location info
        visitData.Location.IP = v.IPAddress
        // Note: Country, Region, City would need to be added to the Visit model
        // and populated using IP geolocation

        // Page info
        visitData.Page.URL = v.LandingPage
        visitData.Page.Referrer = v.Referrer

        // UTM info
        visitData.UTM.Source = v.UTMSource
        visitData.UTM.Medium = v.UTMMedium
        visitData.UTM.Campaign = v.UTMCampaign
        visitData.UTM.Content = v.UTMContent
        visitData.UTM.Term = v.UTMTerm

        // Meta info
        visitData.Meta.Language = v.Language
        visitData.Meta.Timezone = v.Timezone
        visitData.Meta.CreatedAt = v.CreatedAt

        // Conversions
        for _, conv := range v.Conversions {
            convData := ConversionData{
                ID:        conv.ID,
                Amount:    conv.Amount,
                Status:    conv.Status,
                CreatedAt: conv.CreatedAt,
            }
            visitData.Conversions = append(visitData.Conversions, convData)
            stats.Revenue += conv.Amount
        }

        stats.RecentVisits = append(stats.RecentVisits, visitData)
    }

    // Get campaign stats
    stats.Campaigns, err = s.db.GetCampaignStats()
    if err != nil {
        http.Error(w, "Error getting campaign stats", http.StatusInternalServerError)
        return
    }

    // Calculate totals
    stats.TotalVisits = int64(len(recentVisits))
    for _, visit := range recentVisits {
        stats.TotalConversions += int64(len(visit.Conversions))
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(stats)
}

func (s *Server) GetAllStats(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    stats, err := s.db.GetAllStats()
    if err != nil {
        http.Error(w, "Error getting stats", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(stats)
} 