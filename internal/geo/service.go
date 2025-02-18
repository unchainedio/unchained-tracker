package geo

import (
    "encoding/json"
    "fmt"
    "net"
    "net/http"
)

type Service struct {
    client *http.Client
}

type ipAPIResponse struct {
    Country     string `json:"country_code"`
    Region      string `json:"region"`
    City        string `json:"city"`
}

func NewService(_ string) (*Service, error) {
    return &Service{
        client: &http.Client{},
    }, nil
}

func (s *Service) Close() {
    // Nothing to close with HTTP client
}

func (s *Service) GetLocation(ipStr string) (string, string, string, error) {
    // Clean IP address (remove port if present)
    ip := net.ParseIP(ipStr)
    if ip == nil {
        // Try to handle [::1]:port format
        host, _, err := net.SplitHostPort(ipStr)
        if err != nil {
            return "", "", "", fmt.Errorf("invalid IP address: %s", ipStr)
        }
        ip = net.ParseIP(host)
        if ip == nil {
            return "", "", "", fmt.Errorf("invalid IP address: %s", host)
        }
    }

    // Don't lookup location for private/local IPs
    if ip.IsPrivate() || ip.IsLoopback() {
        return "LO", "Local", "Local", nil
    }

    // Use ip-api.com (free, no API key required)
    resp, err := s.client.Get(fmt.Sprintf("http://ip-api.com/json/%s?fields=country_code,region,city", ip.String()))
    if err != nil {
        return "", "", "", err
    }
    defer resp.Body.Close()

    var result ipAPIResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", "", "", err
    }

    return result.Country, result.Region, result.City, nil
} 