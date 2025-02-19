package db

import (
    "database/sql"
    "time"
    "fmt"
    "strings"
    "log"
    "math/rand"
)

func (db *Database) SaveVisit(v *Visit) error {
    query := `
        INSERT INTO visit (
            visitor_id, click_id, campaign_id, ip_address, user_agent,
            browser, browser_version, os, device_type, screen_resolution,
            viewport_size, language, timezone, landing_page, referrer,
            utm_source, utm_medium, utm_campaign, utm_content, utm_term,
            country, region, city,
            created_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
    
    _, err := db.Exec(query,
        v.VisitorID, v.ClickID, v.CampaignID, v.IPAddress, v.UserAgent,
        v.Browser, v.BrowserVersion, v.OS, v.DeviceType, v.ScreenResolution,
        v.ViewportSize, v.Language, v.Timezone, v.LandingPage, v.Referrer,
        v.UTMSource, v.UTMMedium, v.UTMCampaign, v.UTMContent, v.UTMTerm,
        v.Country, v.Region, v.City,
        v.CreatedAt,
    )
    return err
}

func (db *Database) SaveConversion(c *Conversion) error {
    query := `
        INSERT INTO conversion (
            visitor_id, click_id, campaign_id, amount, status, created_at
        ) VALUES (?, ?, ?, ?, ?, ?)
    `
    
    result, err := db.Exec(query,
        c.VisitorID, c.ClickID, c.CampaignID, c.Amount, c.Status, c.CreatedAt,
    )
    if err != nil {
        return err
    }

    id, err := result.LastInsertId()
    if err != nil {
        return err
    }

    c.ID = id
    return nil
}

func (db *Database) GetVisitsByDate(start, end time.Time) ([]*Visit, error) {
    query := `
        SELECT * FROM visit 
        WHERE created_at BETWEEN ? AND ?
        ORDER BY created_at DESC
    `
    
    rows, err := db.Query(query, start, end)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var visits []*Visit
    for rows.Next() {
        v := new(Visit)
        err := rows.Scan(
            &v.ID, &v.VisitorID, &v.ClickID, &v.CampaignID, &v.IPAddress,
            &v.UserAgent, &v.Browser, &v.BrowserVersion, &v.OS, &v.DeviceType,
            &v.ScreenResolution, &v.ViewportSize, &v.Language, &v.Timezone,
            &v.LandingPage, &v.Referrer, &v.UTMSource, &v.UTMMedium,
            &v.UTMCampaign, &v.UTMContent, &v.UTMTerm, &v.CreatedAt,
        )
        if err != nil {
            return nil, err
        }
        visits = append(visits, v)
    }
    return visits, nil
}

func (db *Database) GetVisitCountSince(t time.Time) (int64, error) {
    var count int64
    err := db.QueryRow("SELECT COUNT(*) FROM visit WHERE created_at >= ?", t).Scan(&count)
    return count, err
}

func (db *Database) GetRecentVisitsWithConversions(limit int) ([]*Visit, error) {
    query := `
        SELECT v.*, c.id, c.amount, c.status, c.created_at
        FROM visit v
        LEFT JOIN conversion c ON v.visitor_id = c.visitor_id
        ORDER BY v.created_at DESC
        LIMIT ?
    `
    
    rows, err := db.Query(query, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    visits := make(map[int64]*Visit)
    for rows.Next() {
        v := new(Visit)
        var conv Conversion
        var convID sql.NullInt64
        var convAmount sql.NullFloat64
        var convStatus sql.NullString
        var convCreatedAt sql.NullTime

        err := rows.Scan(
            &v.ID, &v.VisitorID, &v.ClickID, &v.CampaignID, &v.IPAddress,
            &v.UserAgent, &v.Browser, &v.BrowserVersion, &v.OS, &v.DeviceType,
            &v.ScreenResolution, &v.ViewportSize, &v.Language, &v.Timezone,
            &v.LandingPage, &v.Referrer, &v.UTMSource, &v.UTMMedium,
            &v.UTMCampaign, &v.UTMContent, &v.UTMTerm, &v.CreatedAt,
            &convID, &convAmount, &convStatus, &convCreatedAt,
        )
        if err != nil {
            return nil, err
        }

        if visit, exists := visits[v.ID]; exists {
            if convID.Valid {
                conv.ID = convID.Int64
                conv.Amount = convAmount.Float64
                conv.Status = convStatus.String
                conv.CreatedAt = convCreatedAt.Time
                visit.Conversions = append(visit.Conversions, conv)
            }
        } else {
            if convID.Valid {
                conv.ID = convID.Int64
                conv.Amount = convAmount.Float64
                conv.Status = convStatus.String
                conv.CreatedAt = convCreatedAt.Time
                v.Conversions = []Conversion{conv}
            }
            visits[v.ID] = v
        }
    }

    result := make([]*Visit, 0, len(visits))
    for _, v := range visits {
        result = append(result, v)
    }
    return result, nil
}

func (db *Database) GetCampaignStats() ([]CampaignStats, error) {
    log.Printf("Getting campaign stats")
    query := `
        SELECT 
            c.id,
            c.name,
            c.campaign_id,
            c.campaign_token,
            COALESCE(lp.url, '') as landing_page,
            c.traffic_source,
            DATE_FORMAT(c.created_at, '%Y-%m-%d %H:%i:%s') as created_at,
            COUNT(DISTINCT v.id) as visits,
            COUNT(DISTINCT conv.id) as conversions,
            COALESCE(SUM(conv.amount), 0) as revenue
        FROM campaign c
        LEFT JOIN landing_page lp ON c.landing_page_id = lp.id
        LEFT JOIN visit v ON c.campaign_id = v.campaign_id
        LEFT JOIN conversion conv ON v.visitor_id = conv.visitor_id
        GROUP BY c.id, c.name, c.campaign_id, c.campaign_token,
                 lp.url, c.traffic_source, c.created_at
        ORDER BY c.created_at DESC
    `
    
    log.Printf("Running query: %s", query)
    rows, err := db.Query(query)
    if err != nil {
        log.Printf("Error querying campaigns: %v", err)
        return nil, err
    }
    defer rows.Close()

    var stats []CampaignStats
    for rows.Next() {
        var s CampaignStats
        var createdAtStr string
        err := rows.Scan(
            &s.ID, &s.Name, &s.CampaignID, &s.CampaignToken,
            &s.LandingPage, &s.TrafficSource, &createdAtStr,
            &s.Visits, &s.Conversions, &s.Revenue,
        )
        if err != nil {
            return nil, err
        }
        // Parse the timestamp
        s.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAtStr)
        if err != nil {
            return nil, fmt.Errorf("error parsing timestamp: %v", err)
        }
        stats = append(stats, s)
    }
    return stats, nil
}

func (db *Database) GetConversionsByIDs(convIDsStr string) ([]Conversion, error) {
    query := `
        SELECT id, visitor_id, click_id, campaign_id, amount, status, created_at 
        FROM conversion 
        WHERE id IN (?)
    `
    
    // Split comma-separated IDs
    convIDs := strings.Split(convIDsStr, ",")
    
    rows, err := db.Query(query, convIDs)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var conversions []Conversion
    for rows.Next() {
        var conv Conversion
        err := rows.Scan(
            &conv.ID, &conv.VisitorID, &conv.ClickID, 
            &conv.CampaignID, &conv.Amount, &conv.Status, 
            &conv.CreatedAt,
        )
        if err != nil {
            return nil, err
        }
        conversions = append(conversions, conv)
    }
    
    return conversions, nil
}

func (db *Database) GetVisitByClickID(clickID string) (*Visit, error) {
    log.Printf("DB: Looking for visit with click_id: %s", clickID)
    
    query := `
        SELECT 
            id, visitor_id, click_id, campaign_id,
            ip_address, user_agent, browser, browser_version,
            os, device_type, screen_resolution, viewport_size,
            language, timezone, landing_page, referrer,
            utm_source, utm_medium, utm_campaign, utm_content,
            utm_term, DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s') as created_at
        FROM visit 
        WHERE click_id = ? 
        ORDER BY created_at DESC 
        LIMIT 1
    `
    
    var createdAtStr string
    visit := new(Visit)
    err := db.QueryRow(query, clickID).Scan(
        &visit.ID, &visit.VisitorID, &visit.ClickID, &visit.CampaignID,
        &visit.IPAddress, &visit.UserAgent, &visit.Browser, &visit.BrowserVersion,
        &visit.OS, &visit.DeviceType, &visit.ScreenResolution, &visit.ViewportSize,
        &visit.Language, &visit.Timezone, &visit.LandingPage, &visit.Referrer,
        &visit.UTMSource, &visit.UTMMedium, &visit.UTMCampaign, &visit.UTMContent,
        &visit.UTMTerm, &createdAtStr,
    )
    
    if err == sql.ErrNoRows {
        log.Printf("DB: No visit found for click_id: %s", clickID)
        return nil, fmt.Errorf("no visit found for click_id: %s", clickID)
    }
    if err != nil {
        log.Printf("DB: Error querying visit: %v", err)
        return nil, err
    }

    // Parse the timestamp
    visit.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAtStr)
    if err != nil {
        log.Printf("DB: Error parsing timestamp: %v", err)
        return nil, err
    }

    log.Printf("DB: Found visit: %+v", visit)
    return visit, nil
}

func generateCampaignToken() string {
    // Generate 10-digit number
    return fmt.Sprintf("%010d", rand.Int63n(10000000000))
}

func (db *Database) SaveCampaign(c *Campaign) error {
    log.Printf("Saving campaign: %+v", c)
    
    c.CampaignToken = generateCampaignToken()
    query := `
        INSERT INTO campaign (
            name, campaign_id, campaign_token, offer_url,
            traffic_source, created_at
        ) VALUES (?, ?, ?, ?, ?, ?)
    `
    
    result, err := db.Exec(query,
        c.Name, c.CampaignID, c.CampaignToken, c.OfferURL,
        c.TrafficSource, c.CreatedAt,
    )
    if err != nil {
        log.Printf("Database error: %v", err)
        return err
    }
    
    id, err := result.LastInsertId()
    if err != nil {
        return err
    }
    
    c.ID = id
    return nil
}

func (db *Database) DeleteCampaign(campaignID string) error {
    // First check if there are any visits or conversions
    var visitCount int
    err := db.QueryRow("SELECT COUNT(*) FROM visit WHERE campaign_id = ?", campaignID).Scan(&visitCount)
    if err != nil {
        return err
    }

    if visitCount > 0 {
        return fmt.Errorf("cannot delete campaign with existing visits")
    }

    // Delete the campaign if no visits exist
    _, err = db.Exec("DELETE FROM campaign WHERE campaign_id = ?", campaignID)
    return err
}

type FullStats struct {
    Conversions []struct {
        ID          int64     `json:"id"`
        VisitorID   string    `json:"visitor_id"`
        CampaignID  string    `json:"campaign_id"`
        Amount      float64   `json:"amount"`
        Status      string    `json:"status"`
        CreatedAt   string    `json:"created_at"`
        Campaign    string    `json:"campaign_name"`
        VisitorInfo struct {
            Browser     string `json:"browser"`
            OS         string `json:"os"`
            DeviceType string `json:"device_type"`
            IPAddress  string `json:"ip_address"`
        } `json:"visitor_info"`
    } `json:"conversions"`
    Summary struct {
        TotalConversions int64   `json:"total_conversions"`
        TotalRevenue     float64 `json:"total_revenue"`
        AverageAmount    float64 `json:"average_amount"`
    } `json:"summary"`
}

func (db *Database) GetAllStats() (*FullStats, error) {
    query := `
        SELECT 
            c.id,
            c.visitor_id,
            c.campaign_id,
            c.amount,
            c.status,
            DATE_FORMAT(c.created_at, '%Y-%m-%d %H:%i:%s') as created_at,
            COALESCE(camp.name, 'Unknown') as campaign_name,
            COALESCE(v.browser, 'Unknown') as browser,
            COALESCE(v.os, 'Unknown') as os,
            COALESCE(v.device_type, 'Unknown') as device_type,
            COALESCE(v.ip_address, 'Unknown') as ip_address
        FROM conversion c
        LEFT JOIN visit v ON c.visitor_id = v.visitor_id
        LEFT JOIN campaign camp ON c.campaign_id = camp.campaign_id
        ORDER BY c.created_at DESC
    `

    fmt.Printf("Executing query: %s\n", query)

    rows, err := db.Query(query)
    if err != nil {
        fmt.Printf("Query error: %v\n", err)
        return nil, err
    }
    defer rows.Close()

    stats := &FullStats{
        Conversions: make([]struct {
            ID          int64     `json:"id"`
            VisitorID   string    `json:"visitor_id"`
            CampaignID  string    `json:"campaign_id"`
            Amount      float64   `json:"amount"`
            Status      string    `json:"status"`
            CreatedAt   string    `json:"created_at"`
            Campaign    string    `json:"campaign_name"`
            VisitorInfo struct {
                Browser     string `json:"browser"`
                OS         string `json:"os"`
                DeviceType string `json:"device_type"`
                IPAddress  string `json:"ip_address"`
            } `json:"visitor_info"`
        }, 0),
        Summary: struct {
            TotalConversions int64   `json:"total_conversions"`
            TotalRevenue     float64 `json:"total_revenue"`
            AverageAmount    float64 `json:"average_amount"`
        }{
            TotalConversions: 0,
            TotalRevenue:     0,
            AverageAmount:    0,
        },
    }

    for rows.Next() {
        var conv struct {
            ID          int64     `json:"id"`
            VisitorID   string    `json:"visitor_id"`
            CampaignID  string    `json:"campaign_id"`
            Amount      float64   `json:"amount"`
            Status      string    `json:"status"`
            CreatedAt   string    `json:"created_at"`
            Campaign    string    `json:"campaign_name"`
            VisitorInfo struct {
                Browser     string `json:"browser"`
                OS         string `json:"os"`
                DeviceType string `json:"device_type"`
                IPAddress  string `json:"ip_address"`
            } `json:"visitor_info"`
        }
        err := rows.Scan(
            &conv.ID, &conv.VisitorID, &conv.CampaignID, &conv.Amount, &conv.Status,
            &conv.CreatedAt, &conv.Campaign, &conv.VisitorInfo.Browser, &conv.VisitorInfo.OS,
            &conv.VisitorInfo.DeviceType, &conv.VisitorInfo.IPAddress,
        )
        if err != nil {
            return nil, err
        }
        stats.Conversions = append(stats.Conversions, conv)
        stats.Summary.TotalConversions++
        stats.Summary.TotalRevenue += conv.Amount
        stats.Summary.AverageAmount = stats.Summary.TotalRevenue / float64(stats.Summary.TotalConversions)
    }

    return stats, nil
}

func (db *Database) GetVisitByVisitorID(visitorID string) (*Visit, error) {
    log.Printf("DB: Looking for visit with visitor_id: %s", visitorID)
    
    query := `
        SELECT 
            id, visitor_id, click_id, campaign_id,
            ip_address, user_agent, browser, browser_version,
            os, device_type, screen_resolution, viewport_size,
            language, timezone, landing_page, referrer,
            utm_source, utm_medium, utm_campaign, utm_content,
            utm_term, DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s') as created_at
        FROM visit 
        WHERE visitor_id = ? 
        ORDER BY created_at DESC 
        LIMIT 1
    `
    
    var createdAtStr string
    visit := new(Visit)
    err := db.QueryRow(query, visitorID).Scan(
        &visit.ID, &visit.VisitorID, &visit.ClickID, &visit.CampaignID,
        &visit.IPAddress, &visit.UserAgent, &visit.Browser, &visit.BrowserVersion,
        &visit.OS, &visit.DeviceType, &visit.ScreenResolution, &visit.ViewportSize,
        &visit.Language, &visit.Timezone, &visit.LandingPage, &visit.Referrer,
        &visit.UTMSource, &visit.UTMMedium, &visit.UTMCampaign, &visit.UTMContent,
        &visit.UTMTerm, &createdAtStr,
    )
    
    if err == sql.ErrNoRows {
        log.Printf("DB: No visit found for visitor_id: %s", visitorID)
        return nil, fmt.Errorf("no visit found for visitor_id: %s", visitorID)
    }
    if err != nil {
        log.Printf("DB: Error querying visit by visitor_id: %v", err)
        return nil, err
    }

    // Parse the timestamp
    visit.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAtStr)
    if err != nil {
        log.Printf("DB: Error parsing timestamp: %v", err)
        return nil, err
    }

    log.Printf("DB: Found visit: %+v", visit)
    return visit, nil
}

func (db *Database) SaveOffer(o *Offer) error {
    // First check if offer exists
    var exists bool
    err := db.QueryRow(`
        SELECT COUNT(*) > 0 
        FROM offer 
        WHERE name = ? AND network = ?
    `, o.Name, o.Network).Scan(&exists)
    
    if err != nil {
        return err
    }
    
    if exists {
        return fmt.Errorf("offer with name '%s' already exists for network '%s'", o.Name, o.Network)
    }

    // If not exists, insert the new offer
    query := `
        INSERT INTO offer (name, network, offer_url)
        VALUES (?, ?, ?)
    `
    
    result, err := db.Exec(query, o.Name, o.Network, o.OfferURL)
    if err != nil {
        return err
    }

    id, err := result.LastInsertId()
    if err != nil {
        return err
    }

    o.ID = id
    return nil
}

func (db *Database) GetOffers() ([]*Offer, error) {
    query := `
        SELECT 
            id, name, network, offer_url,
            DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s') as created_at
        FROM offer
        ORDER BY created_at DESC
    `
    
    rows, err := db.Query(query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var offers []*Offer
    for rows.Next() {
        o := new(Offer)
        var createdAtStr string
        err := rows.Scan(&o.ID, &o.Name, &o.Network, &o.OfferURL, &createdAtStr)
        if err != nil {
            return nil, err
        }
        
        // Parse the timestamp
        o.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAtStr)
        if err != nil {
            return nil, err
        }
        
        offers = append(offers, o)
    }
    return offers, nil
}

func (db *Database) GetLandingPages() ([]*LandingPage, error) {
    query := `
        SELECT id, name, url, created_at
        FROM landing_page
        ORDER BY created_at DESC
    `
    
    rows, err := db.Query(query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var pages []*LandingPage
    for rows.Next() {
        p := new(LandingPage)
        var createdAtStr string
        err := rows.Scan(&p.ID, &p.Name, &p.URL, &createdAtStr)
        if err != nil {
            return nil, err
        }
        // Parse the timestamp
        p.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAtStr)
        if err != nil {
            return nil, err
        }
        pages = append(pages, p)
    }
    return pages, nil
}

func (db *Database) SaveLandingPage(p *LandingPage) error {
    query := `
        INSERT INTO landing_page (name, url)
        VALUES (?, ?)
    `
    
    result, err := db.Exec(query, p.Name, p.URL)
    if err != nil {
        return err
    }

    id, err := result.LastInsertId()
    if err != nil {
        return err
    }

    p.ID = id
    return nil
}

func (db *Database) UpdateLandingPage(p *LandingPage) error {
    query := `
        UPDATE landing_page 
        SET name = ?, url = ?
        WHERE id = ?
    `
    
    _, err := db.Exec(query, p.Name, p.URL, p.ID)
    return err
}

func (db *Database) DeleteLandingPage(id int64) error {
    query := "DELETE FROM landing_page WHERE id = ?"
    _, err := db.Exec(query, id)
    return err
}

func (db *Database) GetCampaignByToken(token string) (*Campaign, error) {
    query := `
        SELECT id, name, campaign_id, campaign_token, offer_url, landing_page, traffic_source, created_at
        FROM campaign
        WHERE campaign_token = ?
    `
    
    campaign := new(Campaign)
    var createdAtStr string
    err := db.QueryRow(query, token).Scan(
        &campaign.ID, &campaign.Name, &campaign.CampaignID, &campaign.CampaignToken,
        &campaign.OfferURL, &campaign.LandingPage, &campaign.TrafficSource, &createdAtStr,
    )
    if err != nil {
        return nil, err
    }

    campaign.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAtStr)
    if err != nil {
        return nil, err
    }

    return campaign, nil
}

func (db *Database) SaveClick(c *Click) error {
    query := `
        INSERT INTO click (
            click_id, visitor_id, campaign_token, campaign_id,
            ip_address, user_agent, referrer
        ) VALUES (?, ?, ?, ?, ?, ?, ?)
    `
    
    _, err := db.Exec(query,
        c.ClickID, c.VisitorID, c.CampaignToken, c.CampaignID,
        c.IPAddress, c.UserAgent, c.Referrer,
    )
    return err
}

func (db *Database) SaveTrackingDomain(d *TrackingDomain) error {
    query := `
        INSERT INTO tracking_domain (domain, cloudflare_zone_id)
        VALUES (?, ?)
    `
    
    result, err := db.Exec(query, d.Domain, d.CloudflareZoneID)
    if err != nil {
        return err
    }

    id, err := result.LastInsertId()
    if err != nil {
        return err
    }

    d.ID = id
    return nil
}

func (db *Database) GetTrackingDomains() ([]*TrackingDomain, error) {
    query := `
        SELECT id, domain, cloudflare_zone_id, created_at
        FROM tracking_domain
        ORDER BY created_at DESC
    `
    
    rows, err := db.Query(query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var domains []*TrackingDomain
    for rows.Next() {
        d := new(TrackingDomain)
        var createdAtStr string
        err := rows.Scan(&d.ID, &d.Domain, &d.CloudflareZoneID, &createdAtStr)
        if err != nil {
            return nil, err
        }
        d.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAtStr)
        if err != nil {
            return nil, err
        }
        domains = append(domains, d)
    }
    return domains, nil
}