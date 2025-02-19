package main

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "strings"
    "time"
    "github.com/google/uuid"
    
    "unchained-tracker/internal/api"
    "unchained-tracker/internal/config"
    "unchained-tracker/internal/db"
    "unchained-tracker/internal/geo"
    "unchained-tracker/internal/db/migrations"
)

// Helper functions
func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

func average(nums []float64) float64 {
    if len(nums) == 0 {
        return 0
    }
    sum := 0.0
    for _, n := range nums {
        sum += n
    }
    return sum / float64(len(nums))
}

func min64(nums []float64) float64 {
    if len(nums) == 0 {
        return 0
    }
    min := nums[0]
    for _, n := range nums {
        if n < min {
            min = n
        }
    }
    return min
}

func max64(nums []float64) float64 {
    if len(nums) == 0 {
        return 0
    }
    max := nums[0]
    for _, n := range nums {
        if n > max {
            max = n
        }
    }
    return max
}

func main() {
    // Load configuration
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Initialize database
    database, err := db.Connect(cfg.DatabaseURL)
    if err != nil {
        log.Fatalf("Failed to connect to database: %v", err)
    }
    defer database.Close()

    // Run migrations
    if err := migrations.Run(database.DB()); err != nil {
        log.Fatalf("Failed to run migrations: %v", err)
    }

    // Initialize geolocation service
    geo, err := geo.NewService("GeoLite2-City.mmdb")
    if err != nil {
        log.Printf("Warning: Geolocation service not available: %v", err)
    }
    defer geo.Close()

    // Create API server with geo service
    server := api.NewServer(database, cfg, geo)

    // Create router
    mux := http.NewServeMux()
    
    // Static file serving
    fileServer := http.FileServer(http.Dir("static"))
    
    // Create a custom file server handler
    customFileServer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Set correct MIME types
        if strings.HasSuffix(r.URL.Path, ".css") {
            w.Header().Set("Content-Type", "text/css")
        } else if strings.HasSuffix(r.URL.Path, ".js") {
            w.Header().Set("Content-Type", "application/javascript")
        } else if strings.HasSuffix(r.URL.Path, ".ico") {
            w.Header().Set("Content-Type", "image/x-icon")
        }
        r.URL.Path = strings.TrimPrefix(r.URL.Path, "/static/")
        fileServer.ServeHTTP(w, r)
    })
    
    // Handle all static files
    mux.Handle("/static/", customFileServer)
    
    // Serve favicon
    mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
        http.ServeFile(w, r, "static/favicon.ico")
    })
    
    // Serve HTML files
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/" {
            http.ServeFile(w, r, "static/dashboard.html")
            return
        }
        if r.URL.Path == "/stats" {
            server.GetAllStats(w, r)
            return
        }
        if r.URL.Path == "/test" {
            http.ServeFile(w, r, "static/test.html")
            return
        }
        http.NotFound(w, r)
    })
    
    mux.HandleFunc("/campaigns", func(w http.ResponseWriter, r *http.Request) {
        http.ServeFile(w, r, "static/campaigns.html")
    })
    
    // API routes
    mux.HandleFunc("/track", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        err := server.HandleVisit(w, r)
        if err != nil {
            json.NewEncoder(w).Encode(map[string]interface{}{
                "error": err.Error(),
                "status": "error",
            })
            return
        }
    })
    mux.HandleFunc("/click", func(w http.ResponseWriter, r *http.Request) {
        // Get campaign token from query
        token := r.URL.Query().Get("rtkck")
        if token == "" {
            http.Error(w, "Missing campaign token", http.StatusBadRequest)
            return
        }

        // Look up campaign by token
        var campaignID string
        var offerURL string
        log.Printf("Looking up campaign with token: %s", token)
        err := database.QueryRow(
            "SELECT campaign_id, offer_url FROM campaign WHERE campaign_token = ?", 
            token,
        ).Scan(&campaignID, &offerURL)
        if err != nil {
            log.Printf("Error finding campaign: %v", err)
            http.Error(w, "Invalid campaign token", http.StatusBadRequest)
            return
        }
        log.Printf("Found campaign: id=%s, offer_url=%s", campaignID, offerURL)

        // Generate click ID
        clickID := fmt.Sprintf("%x", time.Now().UnixNano())

        // Save click
        _, err = database.Exec(`
            INSERT INTO click (
                click_id, visitor_id, campaign_token, 
                campaign_id, ip_address, user_agent, referrer
            ) VALUES (?, ?, ?, ?, ?, ?, ?)`,
            clickID,
            uuid.New().String(), // Generate visitor ID
            token,
            campaignID,
            r.RemoteAddr,
            r.UserAgent(),
            r.Referer(),
        )
        if err != nil {
            log.Printf("Error saving click: %v", err)
            http.Error(w, "Error tracking click", http.StatusInternalServerError)
            return
        }

        // Redirect to offer URL with click ID
        redirectURL := fmt.Sprintf("%s?clickid=%s&source=%s", 
            offerURL, clickID, campaignID)
        http.Redirect(w, r, redirectURL, http.StatusFound)
    })
    mux.HandleFunc("/postback", server.HandleConversion)
    mux.HandleFunc("/network/postback", server.HandleNetworkPostback)
    mux.HandleFunc("/api/campaigns", server.HandleCampaigns)
    mux.HandleFunc("/api/dashboard/stats", server.GetDashboardStats)

    // Single debug endpoint that combines all debug information
    mux.HandleFunc("/debug", func(w http.ResponseWriter, r *http.Request) {
        debug := make(map[string]interface{})

        // 1. Get table counts
        counts := make(map[string]int)
        tables := []string{"visit", "conversion", "campaign"}
        for _, table := range tables {
            var count int
            err := database.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
            if err != nil {
                log.Printf("Error getting count for %s: %v", table, err)
                counts[table] = -1
            } else {
                counts[table] = count
            }
        }
        debug["counts"] = counts

        // 2. Get all visits with full details
        visitRows, err := database.Query(`
            SELECT 
                v.id,
                v.visitor_id,
                v.click_id,
                v.campaign_id,
                v.ip_address,
                v.browser,
                v.os,
                v.device_type,
                v.screen_resolution,
                v.viewport_size,
                DATE_FORMAT(v.created_at, '%Y-%m-%d %H:%i:%s') as created_at,
                COUNT(c.id) as conversion_count,
                GROUP_CONCAT(c.id) as conversion_ids,
                GROUP_CONCAT(CAST(c.amount as CHAR)) as conversion_amounts,
                GROUP_CONCAT(DATE_FORMAT(c.created_at, '%Y-%m-%d %H:%i:%s')) as conversion_times
            FROM visit v 
            LEFT JOIN conversion c ON v.visitor_id = c.visitor_id 
            GROUP BY v.id
            ORDER BY v.created_at DESC
            LIMIT 100
        `)
        if err != nil {
            log.Printf("Error querying visits: %v", err)
        } else {
            defer visitRows.Close()
            visits := make([]map[string]interface{}, 0)
            cols, _ := visitRows.Columns()
            for visitRows.Next() {
                values := make([]interface{}, len(cols))
                valuePtrs := make([]interface{}, len(cols))
                for i := range values {
                    valuePtrs[i] = &values[i]
                }
                if err := visitRows.Scan(valuePtrs...); err != nil {
                    log.Printf("Error scanning visit: %v", err)
                    continue
                }
                row := make(map[string]interface{})
                for i, col := range cols {
                    if values[i] == nil {
                        continue
                    }
                    switch v := values[i].(type) {
                    case []byte:
                        row[col] = string(v)
                    default:
                        row[col] = v
                    }
                }
                visits = append(visits, row)
            }
            debug["visits"] = visits
        }

        // 3. Get all conversions with full details
        convRows, err := database.Query(`
            SELECT 
                c.id,
                c.visitor_id,
                c.click_id,
                c.campaign_id,
                CAST(c.amount as DECIMAL(10,2)) as amount,
                c.status,
                DATE_FORMAT(c.created_at, '%Y-%m-%d %H:%i:%s') as created_at,
                v.browser,
                v.os,
                v.device_type,
                v.screen_resolution,
                v.viewport_size,
                v.ip_address,
                camp.name as campaign_name,
                TIMESTAMPDIFF(SECOND, v.created_at, c.created_at) as seconds_to_convert
            FROM conversion c
            LEFT JOIN visit v ON c.visitor_id = v.visitor_id
            LEFT JOIN campaign camp ON c.campaign_id = camp.campaign_id
            ORDER BY c.created_at DESC
            LIMIT 100
        `)
        if err != nil {
            log.Printf("Error querying conversions: %v", err)
        } else {
            defer convRows.Close()
            conversions := make([]map[string]interface{}, 0)
            cols, _ := convRows.Columns()
            for convRows.Next() {
                values := make([]interface{}, len(cols))
                valuePtrs := make([]interface{}, len(cols))
                for i := range values {
                    valuePtrs[i] = &values[i]
                }
                if err := convRows.Scan(valuePtrs...); err != nil {
                    log.Printf("Error scanning conversion: %v", err)
                    continue
                }
                row := make(map[string]interface{})
                for i, col := range cols {
                    if values[i] == nil {
                        continue
                    }
                    switch v := values[i].(type) {
                    case []byte:
                        row[col] = string(v)
                    default:
                        row[col] = v
                    }
                }
                conversions = append(conversions, row)
            }
            debug["conversions"] = conversions
        }

        // 4. Calculate summary statistics
        summary := map[string]interface{}{
            "table_counts": counts,
            "data_health": map[string]interface{}{
                "visits_without_conversions": 0,
                "conversions_without_visits": 0,
                "multiple_conversions_per_visit": 0,
                "total_revenue": 0.0,
                "average_conversion_value": 0.0,
                "conversion_rate": 0.0,
            },
            "recent_activity": map[string]interface{}{
                "last_visit_time": nil,
                "last_conversion_time": nil,
                "last_24h_visits": 0,
                "last_24h_conversions": 0,
            },
            "errors": map[string]interface{}{
                "orphaned_conversions": 0,
                "invalid_campaigns": 0,
                "duplicate_visits": 0,
            },
        }

        // Calculate data health metrics
        if visits, ok := debug["visits"].([]map[string]interface{}); ok {
            for _, visit := range visits {
                convCount, _ := visit["conversion_count"].(int64)
                if convCount == 0 {
                    summary["data_health"].(map[string]interface{})["visits_without_conversions"] = 
                        summary["data_health"].(map[string]interface{})["visits_without_conversions"].(int) + 1
                } else if convCount > 1 {
                    summary["data_health"].(map[string]interface{})["multiple_conversions_per_visit"] = 
                        summary["data_health"].(map[string]interface{})["multiple_conversions_per_visit"].(int) + 1
                }
            }
        }

        if conversions, ok := debug["conversions"].([]map[string]interface{}); ok {
            totalRevenue := 0.0
            for _, conv := range conversions {
                if amount, ok := conv["amount"].(float64); ok {
                    totalRevenue += amount
                }
            }
            summary["data_health"].(map[string]interface{})["total_revenue"] = totalRevenue
            if len(conversions) > 0 {
                summary["data_health"].(map[string]interface{})["average_conversion_value"] = totalRevenue / float64(len(conversions))
            }
        }

        if counts["visit"] > 0 {
            summary["data_health"].(map[string]interface{})["conversion_rate"] = 
                float64(counts["conversion"]) / float64(counts["visit"]) * 100
        }

        // Get recent activity
        lastDay := time.Now().Add(-24 * time.Hour)
        
        // Get last visit time and 24h count
        var lastVisitTime sql.NullString
        var last24hVisits int
        err = database.QueryRow(`
            SELECT 
                DATE_FORMAT(MAX(created_at), '%Y-%m-%d %H:%i:%s'),
                COUNT(*) 
            FROM visit 
            WHERE created_at >= ?
        `, lastDay).Scan(&lastVisitTime, &last24hVisits)
        if err != nil {
            log.Printf("Error getting recent visit stats: %v", err)
        } else {
            if lastVisitTime.Valid {
                t, err := time.Parse("2006-01-02 15:04:05", lastVisitTime.String)
                if err == nil {
                    summary["recent_activity"].(map[string]interface{})["last_visit_time"] = t
                }
            }
            summary["recent_activity"].(map[string]interface{})["last_24h_visits"] = last24hVisits
        }

        // Get last conversion time and 24h count
        var lastConvTime sql.NullString
        var last24hConvs int
        err = database.QueryRow(`
            SELECT 
                DATE_FORMAT(MAX(created_at), '%Y-%m-%d %H:%i:%s'),
                COUNT(*) 
            FROM conversion 
            WHERE created_at >= ?
        `, lastDay).Scan(&lastConvTime, &last24hConvs)
        if err != nil {
            log.Printf("Error getting recent conversion stats: %v", err)
        } else {
            if lastConvTime.Valid {
                t, err := time.Parse("2006-01-02 15:04:05", lastConvTime.String)
                if err == nil {
                    summary["recent_activity"].(map[string]interface{})["last_conversion_time"] = t
                }
            }
            summary["recent_activity"].(map[string]interface{})["last_24h_conversions"] = last24hConvs
        }

        // Check for orphaned conversions
        var orphanedConvs int
        err = database.QueryRow(`
            SELECT COUNT(*) 
            FROM conversion c 
            LEFT JOIN visit v ON c.visitor_id = v.visitor_id 
            WHERE v.id IS NULL
        `).Scan(&orphanedConvs)
        if err != nil {
            log.Printf("Error checking orphaned conversions: %v", err)
        } else {
            summary["errors"].(map[string]interface{})["orphaned_conversions"] = orphanedConvs
        }

        // Check for invalid campaigns
        var invalidCampaigns int
        err = database.QueryRow(`
            SELECT COUNT(*) 
            FROM visit v 
            LEFT JOIN campaign c ON v.campaign_id = c.campaign_id 
            WHERE v.campaign_id IS NOT NULL AND c.id IS NULL
        `).Scan(&invalidCampaigns)
        if err != nil {
            log.Printf("Error checking invalid campaigns: %v", err)
        } else {
            summary["errors"].(map[string]interface{})["invalid_campaigns"] = invalidCampaigns
        }

        // Check for duplicate visits
        var duplicateVisits int
        err = database.QueryRow(`
            SELECT COUNT(*) - COUNT(DISTINCT click_id) 
            FROM visit 
            WHERE click_id IS NOT NULL
        `).Scan(&duplicateVisits)
        if err != nil {
            log.Printf("Error checking duplicate visits: %v", err)
        } else {
            summary["errors"].(map[string]interface{})["duplicate_visits"] = duplicateVisits
        }

        // Add recent activity summary
        if conversions, ok := debug["conversions"].([]map[string]interface{}); ok && len(conversions) > 0 {
            summary["recent_activity"].(map[string]interface{})["latest_conversions"] = conversions[:min(5, len(conversions))]
        }
        if visits, ok := debug["visits"].([]map[string]interface{}); ok && len(visits) > 0 {
            summary["recent_activity"].(map[string]interface{})["latest_visits"] = visits[:min(5, len(visits))]
        }

        // Add conversion time analysis
        if conversions, ok := debug["conversions"].([]map[string]interface{}); ok {
            var conversionTimes []float64
            for _, conv := range conversions {
                if seconds, ok := conv["seconds_to_convert"].(float64); ok {
                    conversionTimes = append(conversionTimes, seconds)
                }
            }
            if len(conversionTimes) > 0 {
                summary["data_health"].(map[string]interface{})["avg_seconds_to_convert"] = average(conversionTimes)
                summary["data_health"].(map[string]interface{})["min_seconds_to_convert"] = min64(conversionTimes)
                summary["data_health"].(map[string]interface{})["max_seconds_to_convert"] = max64(conversionTimes)
            }
        }

        debug["summary"] = summary

        // Set response headers and encode
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(debug)
    })

    // Add debug endpoint to check visit by click_id
    mux.HandleFunc("/debug/visit", func(w http.ResponseWriter, r *http.Request) {
        clickID := r.URL.Query().Get("click_id")
        if clickID == "" {
            http.Error(w, "Missing click_id parameter", http.StatusBadRequest)
            return
        }

        rows, err := database.Query(`
            SELECT 
                id, visitor_id, click_id, campaign_id, created_at 
            FROM visit 
            WHERE click_id = ?
        `, clickID)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        defer rows.Close()

        var visits []map[string]interface{}
        for rows.Next() {
            var id int64
            var visitorID, clickID, campaignID string
            var createdAt time.Time
            
            if err := rows.Scan(&id, &visitorID, &clickID, &campaignID, &createdAt); err != nil {
                continue
            }
            
            visits = append(visits, map[string]interface{}{
                "id": id,
                "visitor_id": visitorID,
                "click_id": clickID,
                "campaign_id": campaignID,
                "created_at": createdAt,
            })
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "click_id": clickID,
            "visits": visits,
        })
    })

    // Add SQL debug endpoint
    mux.HandleFunc("/debug/sql", func(w http.ResponseWriter, r *http.Request) {
        query := r.URL.Query().Get("q")
        if query == "" {
            query = "SELECT * FROM visit ORDER BY created_at DESC LIMIT 10"
        }

        rows, err := database.Query(query)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        defer rows.Close()

        // Get column names
        cols, err := rows.Columns()
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        // Prepare result
        var result []map[string]interface{}

        // Prepare values
        values := make([]interface{}, len(cols))
        valuePtrs := make([]interface{}, len(cols))
        for i := range values {
            valuePtrs[i] = &values[i]
        }

        // Scan rows
        for rows.Next() {
            err := rows.Scan(valuePtrs...)
            if err != nil {
                continue
            }

            // Create map of column name to value
            row := make(map[string]interface{})
            for i, col := range cols {
                var v interface{}
                val := values[i]
                b, ok := val.([]byte)
                if ok {
                    v = string(b)
                } else {
                    v = val
                }
                row[col] = v
            }
            result = append(result, row)
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "query": query,
            "rows": result,
        })
    })

    // Add migration endpoint (only in development)
    mux.HandleFunc("/debug/migrate", func(w http.ResponseWriter, r *http.Request) {
        // Add location columns
        for _, column := range []string{"country", "region", "city"} {
            // Check if column exists
            var exists bool
            err := database.QueryRow(`
                SELECT COUNT(*) > 0 
                FROM information_schema.columns 
                WHERE table_name = 'visit' 
                AND column_name = ?
            `, column).Scan(&exists)
            
            if err != nil {
                log.Printf("Error checking column %s: %v", column, err)
                continue
            }

            if !exists {
                // Add column if it doesn't exist
                _, err := database.Exec(fmt.Sprintf(`
                    ALTER TABLE visit
                    ADD COLUMN %s varchar(100) DEFAULT NULL
                `, column))
                
                if err != nil {
                    log.Printf("Error adding column %s: %v", column, err)
                }
            }
        }

        // Verify columns were added
        rows, err := database.Query("SHOW COLUMNS FROM visit")
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        defer rows.Close()

        var columns []string
        for rows.Next() {
            var field, type_, null, key, default_, extra sql.NullString
            if err := rows.Scan(&field, &type_, &null, &key, &default_, &extra); err != nil {
                continue
            }
            columns = append(columns, field.String)
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status": "success",
            "message": "Migration completed",
            "columns": columns,
        })
    })

    // Add offers route
    mux.HandleFunc("/offers", func(w http.ResponseWriter, r *http.Request) {
        http.ServeFile(w, r, "static/offers.html")
    })
    mux.HandleFunc("/api/offers", server.HandleOffers)

    // Landing pages routes
    mux.HandleFunc("/landing-pages", func(w http.ResponseWriter, r *http.Request) {
        http.ServeFile(w, r, "static/landing_pages.html")
    })
    mux.HandleFunc("/api/landing-pages", server.HandleLandingPages)
    mux.HandleFunc("/api/tracking-domains", server.HandleTrackingDomains)

    // Debug endpoints
    mux.HandleFunc("/test-click", func(w http.ResponseWriter, r *http.Request) {
        http.ServeFile(w, r, "static/test-click.html")
    })

    mux.HandleFunc("/debug/clicks", func(w http.ResponseWriter, r *http.Request) {
        log.Printf("Fetching click debug data")
        rows, err := database.Query(`
            SELECT 
                c.id,
                c.click_id,
                c.campaign_token,
                c.campaign_id,
                c.ip_address,
                DATE_FORMAT(c.created_at, '%Y-%m-%d %H:%i:%s') as created_at,
                camp.name as campaign_name,
                COALESCE(camp.offer_url, 'http://localhost:8080/test-offer') as offer_url
            FROM click c
            LEFT JOIN campaign camp ON c.campaign_id = camp.campaign_id
            ORDER BY c.created_at DESC
            LIMIT 10
        `)
        if err != nil {
            log.Printf("Error querying clicks: %v", err)
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        defer rows.Close()

        var clicks []map[string]interface{}
        for rows.Next() {
            var (
                id            int64
                clickID       string
                campaignToken string
                campaignID    string
                ipAddress     string
                createdAt     string
                campaignName  string
                offerURL      string
            )
            
            err := rows.Scan(
                &id,
                &clickID,
                &campaignToken,
                &campaignID,
                &ipAddress,
                &createdAt,
                &campaignName,
                &offerURL,
            )
            if err != nil {
                log.Printf("Error scanning row: %v", err)
                continue
            }
            
            click := map[string]interface{}{
                "id":             id,
                "click_id":       clickID,
                "campaign_token": campaignToken,
                "campaign_id":    campaignID,
                "ip_address":     ipAddress,
                "created_at":     createdAt,
                "campaign_name":  campaignName,
                "offer_url":      offerURL,
            }
            
            clicks = append(clicks, click)
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(clicks)
    })

    // Test endpoints
    mux.HandleFunc("/test-offer", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html")
        fmt.Fprintf(w, `
            <h1>Test Offer Page</h1>
            <p>Click ID: %s</p>
            <p>Campaign: %s</p>
            <pre>%s</pre>
            <hr>
            <h2>Test Actions</h2>
            <button onclick="testConversion()">Track Conversion ($99.99)</button>
            <script>
                async function testConversion() {
                    try {
                        const clickId = new URLSearchParams(window.location.search).get('clickid');
                        const response = await fetch('/postback', {
                            method: 'POST',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify({
                                click_id: clickId,
                                amount: 99.99
                            })
                        });
                        if (response.ok) {
                            alert('Conversion tracked successfully!');
                        } else {
                            alert('Error tracking conversion');
                        }
                    } catch (error) {
                        alert('Error: ' + error.message);
                    }
                }
            </script>
        `,
        r.URL.Query().Get("clickid"),
        r.URL.Query().Get("source"),
        fmt.Sprintf("%+v", r.URL.Query()),
    )
    })

    // Add CORS middleware
    corsMiddleware := func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Access-Control-Allow-Origin", "*")
            w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
            w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
            
            if r.Method == "OPTIONS" {
                w.WriteHeader(http.StatusOK)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }

    // Start server
    log.Printf("Server starting on %s", cfg.ServerAddr)
    if err := http.ListenAndServe(cfg.ServerAddr, corsMiddleware(mux)); err != nil {
        log.Fatalf("Server failed: %v", err)
    }
} 