package db

import (
	"time"
)

type Visit struct {
	ID              int64     `json:"id"`
	VisitorID       string    `json:"visitor_id"`
	ClickID         string    `json:"click_id"`
	CampaignID      string    `json:"campaign_id"`
	IPAddress       string    `json:"ip_address"`
	UserAgent       string    `json:"user_agent"`
	Browser         string    `json:"browser"`
	BrowserVersion  string    `json:"browser_version"`
	OS              string    `json:"os"`
	DeviceType      string    `json:"device_type"`
	ScreenResolution string   `json:"screen_resolution"`
	ViewportSize    string    `json:"viewport_size"`
	Language        string    `json:"language"`
	Timezone        string    `json:"timezone"`
	LandingPage     string    `json:"landing_page"`
	Referrer        string    `json:"referrer"`
	UTMSource       string    `json:"utm_source"`
	UTMMedium       string    `json:"utm_medium"`
	UTMCampaign     string    `json:"utm_campaign"`
	UTMContent      string    `json:"utm_content"`
	UTMTerm         string    `json:"utm_term"`
	CreatedAt       time.Time `json:"created_at"`
	Conversions     []Conversion `json:"conversions,omitempty"`
	Country         string    `json:"country"`
	Region          string    `json:"region"`
	City            string    `json:"city"`
}

type Conversion struct {
	ID          int64     `json:"id"`
	VisitorID   string    `json:"visitor_id"`
	ClickID     string    `json:"click_id"`
	CampaignID  string    `json:"campaign_id"`
	Amount      float64   `json:"amount"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type Campaign struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	CampaignID    string    `json:"campaign_id"`
	LandingPage   string    `json:"landing_page"`
	TrafficSource string    `json:"traffic_source"`
	CreatedAt     time.Time `json:"created_at"`
}

type CampaignStats struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	CampaignID    string    `json:"campaign_id"`
	LandingPage   string    `json:"landing_page"`
	TrafficSource string    `json:"traffic_source"`
	CreatedAt     time.Time `json:"created_at"`
	Visits        int64     `json:"visits"`
	Conversions   int64     `json:"conversions"`
	Revenue       float64   `json:"revenue"`
}

type Offer struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Network   string    `json:"network"`
	OfferURL  string    `json:"offer_url"`
	CreatedAt time.Time `json:"created_at"`
}

type LandingPage struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	URL          string    `json:"url"`
	CreatedAt    time.Time `json:"created_at"`
} 