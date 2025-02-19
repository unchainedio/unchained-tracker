package cloudflare

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type Client struct {
    apiToken string
    baseURL  string
}

type DNSRecord struct {
    Type    string `json:"type"`
    Name    string `json:"name"`
    Content string `json:"content"`
    Proxied bool   `json:"proxied"`
}

func NewClient(apiToken string) *Client {
    return &Client{
        apiToken: apiToken,
        baseURL:  "https://api.cloudflare.com/client/v4",
    }
}

func (c *Client) CreateDNSRecord(zoneID, domain, serverIP string) error {
    record := DNSRecord{
        Type:    "A",
        Name:    domain,
        Content: serverIP,
        Proxied: true,
    }

    body, err := json.Marshal(record)
    if err != nil {
        return err
    }

    url := fmt.Sprintf("%s/zones/%s/dns_records", c.baseURL, zoneID)
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
    if err != nil {
        return err
    }

    req.Header.Set("Authorization", "Bearer "+c.apiToken)
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("cloudflare API error: %d", resp.StatusCode)
    }

    return nil
} 