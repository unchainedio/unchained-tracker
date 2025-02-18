package api

import (
    "testing"
    "net/http/httptest"
    "encoding/json"
    "bytes"
)

func TestTrackVisit(t *testing.T) {
    // Create test server
    server := NewServer(testDB, testConfig)

    // Create test request
    body := map[string]interface{}{
        "campaign_id": "test_campaign",
        "click_id": "test123",
    }
    jsonBody, _ := json.Marshal(body)
    req := httptest.NewRequest("POST", "/track", bytes.NewBuffer(jsonBody))
    w := httptest.NewRecorder()

    // Handle request
    server.HandleVisit(w, req)

    // Check response
    if w.Code != 200 {
        t.Errorf("Expected 200, got %d", w.Code)
    }

    var response map[string]interface{}
    json.NewDecoder(w.Body).Decode(&response)
    if response["status"] != "success" {
        t.Errorf("Expected success status, got %v", response["status"])
    }
} 