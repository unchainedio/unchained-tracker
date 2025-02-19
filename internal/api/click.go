package api

import (
	"net/http"
	"net/url"
	"crypto/rand"
	"time"
	"fmt"
	"unchained-tracker/internal/db"
	"log"
	"strings"
)

func generateClickID() string {
	// Format: timestamp + random hex (24 chars total)
	timestamp := time.Now().Unix()
	random := make([]byte, 8)
	rand.Read(random)
	return fmt.Sprintf("%x%x", timestamp, random)
}

func getVisitorID(r *http.Request) string {
	// First try to get from cookie
	if cookie, err := r.Cookie("visitor_id"); err == nil {
		return cookie.Value
	}

	// If no cookie, generate new ID
	random := make([]byte, 16)
	rand.Read(random)
	return fmt.Sprintf("%x", random)
}

func getIPAddress(r *http.Request) string {
	// Check X-Forwarded-For header
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return strings.Split(forwarded, ",")[0]
	}
	// Fall back to RemoteAddr
	return strings.Split(r.RemoteAddr, ":")[0]
}

func (s *Server) HandleClick(w http.ResponseWriter, r *http.Request) {
	// Get click parameters
	clickID := generateClickID()
	visitorID := getVisitorID(r)
	campaignToken := r.URL.Query().Get("rtkck")

	// Validate campaign token
	campaign, err := s.db.GetCampaignByToken(campaignToken)
	if err != nil {
		http.Error(w, "Invalid campaign", http.StatusBadRequest)
		return
	}

	// Record click
	click := &db.Click{
		ClickID:       clickID,
		VisitorID:     visitorID,
		CampaignToken: campaignToken,
		CampaignID:    campaign.CampaignID,
		IPAddress:     getIPAddress(r),
		UserAgent:     r.UserAgent(),
		Referrer:      r.Referer(),
	}
	
	if err := s.db.SaveClick(click); err != nil {
		log.Printf("Error saving click: %v", err)
		// Continue anyway to not disrupt user experience
	}

	// Set visitor cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "visitor_id",
		Value:    visitorID,
		MaxAge:   86400 * 30,  // 30 days
		Path:     "/",
	})

	// Build redirect URL with parameters
	redirectURL := buildNetworkURL(campaign.OfferURL, click)

	// Perform redirect
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func buildNetworkURL(baseURL string, click *db.Click) string {
	u, _ := url.Parse(baseURL)
	q := u.Query()
	q.Set("clickid", click.ClickID)
	q.Set("aff_id", "YOUR_AFF_ID") // From config
	q.Set("source", click.CampaignID)
	u.RawQuery = q.Encode()
	return u.String()
} 