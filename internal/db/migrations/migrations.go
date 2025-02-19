package migrations

import (
    "database/sql"
    "fmt"
    "strings"
)

type Migration struct {
    Version     int
    Description string
    SQL         string
}

var All = []Migration{
    {
        Version:     1,
        Description: "Create initial tables",
        SQL: `
            /* Create landing pages table first */
            CREATE TABLE IF NOT EXISTS landing_page (
                id INT AUTO_INCREMENT PRIMARY KEY,
                name VARCHAR(100) NOT NULL,
                url VARCHAR(500) NOT NULL,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                UNIQUE KEY unique_url (url)
            );

            /* Then create campaign table with foreign key */
            CREATE TABLE IF NOT EXISTS campaign (
                id INT AUTO_INCREMENT PRIMARY KEY,
                name VARCHAR(100) NOT NULL,
                campaign_id VARCHAR(36) UNIQUE,
                campaign_token VARCHAR(10) UNIQUE,
                offer_url VARCHAR(500),
                landing_page_id INT,
                traffic_source VARCHAR(100),
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                FOREIGN KEY (landing_page_id) REFERENCES landing_page(id)
            );

            CREATE TABLE IF NOT EXISTS visit (
                id INT AUTO_INCREMENT PRIMARY KEY,
                visitor_id VARCHAR(36) UNIQUE,
                click_id VARCHAR(100),
                campaign_id VARCHAR(36),
                ip_address VARCHAR(45),
                user_agent VARCHAR(500),
                browser VARCHAR(100),
                browser_version VARCHAR(50),
                os VARCHAR(100),
                device_type VARCHAR(50),
                screen_resolution VARCHAR(50),
                viewport_size VARCHAR(50),
                language VARCHAR(10),
                timezone VARCHAR(50),
                landing_page VARCHAR(500),
                referrer VARCHAR(500),
                utm_source VARCHAR(100),
                utm_medium VARCHAR(100),
                utm_campaign VARCHAR(100),
                utm_content VARCHAR(100),
                utm_term VARCHAR(100),
                country VARCHAR(2) DEFAULT NULL,
                region VARCHAR(100) DEFAULT NULL,
                city VARCHAR(100) DEFAULT NULL,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                FOREIGN KEY (campaign_id) REFERENCES campaign(campaign_id)
            );

            CREATE TABLE IF NOT EXISTS conversion (
                id INT AUTO_INCREMENT PRIMARY KEY,
                visitor_id VARCHAR(36),
                click_id VARCHAR(255),
                campaign_id VARCHAR(36),
                amount FLOAT,
                status VARCHAR(50) DEFAULT 'pending',
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                FOREIGN KEY (visitor_id) REFERENCES visit(visitor_id),
                FOREIGN KEY (campaign_id) REFERENCES campaign(campaign_id)
            );

            /* Insert test campaign */
            INSERT IGNORE INTO campaign (
                name, 
                campaign_id, 
                campaign_token,
                offer_url,
                landing_page_id,
                traffic_source,
                created_at
            ) VALUES (
                'Test Campaign',
                'test-campaign',
                '1234567890',
                'http://localhost:8080/test-offer',
                NULL,
                'test',
                CURRENT_TIMESTAMP
            );
        `,
    },
    {
        Version:     2,
        Description: "Add offers table",
        SQL: `
            CREATE TABLE IF NOT EXISTS offer (
                id INT NOT NULL AUTO_INCREMENT,
                name VARCHAR(100) NOT NULL,
                network VARCHAR(100) NOT NULL,
                offer_url VARCHAR(500) NOT NULL,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                PRIMARY KEY (id),
                UNIQUE KEY unique_offer (name, network)
            );
        `,
    },
    {
        Version:     3,
        Description: "Add indexes for performance",
        SQL: `
            /* Add indexes for common queries */
            CREATE INDEX idx_visit_created_at ON visit(created_at);
            CREATE INDEX idx_conversion_created_at ON conversion(created_at);
            CREATE INDEX idx_visit_click_id ON visit(click_id);
            CREATE INDEX idx_visit_campaign_id ON visit(campaign_id);
            CREATE INDEX idx_conversion_visitor_id ON conversion(visitor_id);
            CREATE INDEX idx_conversion_campaign_id ON conversion(campaign_id);
        `,
    },
    {
        Version: 6,
        Description: "Add tracking domains and clicks",
        SQL: `
            CREATE TABLE IF NOT EXISTS tracking_domain (
                id INT AUTO_INCREMENT PRIMARY KEY,
                domain VARCHAR(255) NOT NULL UNIQUE,
                cloudflare_zone_id VARCHAR(32),
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP
            );

            CREATE TABLE IF NOT EXISTS click (
                id INT AUTO_INCREMENT PRIMARY KEY,
                click_id VARCHAR(32) NOT NULL UNIQUE,
                visitor_id VARCHAR(36) NOT NULL,
                campaign_token VARCHAR(10) NOT NULL,
                campaign_id VARCHAR(36),
                ip_address VARCHAR(45),
                user_agent VARCHAR(500),
                referrer VARCHAR(500),
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                FOREIGN KEY (campaign_id) REFERENCES campaign(campaign_id)
            );
        `,
    },
}

// Create migrations table if it doesn't exist
const createMigrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INT PRIMARY KEY,
    description TEXT NOT NULL,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)`

func Run(db *sql.DB) error {
    // Create migrations table
    if _, err := db.Exec(createMigrationsTable); err != nil {
        return fmt.Errorf("error creating migrations table: %v", err)
    }

    // Get applied migrations
    applied := make(map[int]bool)
    rows, err := db.Query("SELECT version FROM schema_migrations")
    if err != nil {
        return fmt.Errorf("error checking applied migrations: %v", err)
    }
    defer rows.Close()

    for rows.Next() {
        var version int
        if err := rows.Scan(&version); err != nil {
            return fmt.Errorf("error scanning migration version: %v", err)
        }
        applied[version] = true
    }

    // Run pending migrations
    for _, m := range All {
        if !applied[m.Version] {
            fmt.Printf("Running migration %d: %s\n", m.Version, m.Description)
            
            tx, err := db.Begin()
            if err != nil {
                return fmt.Errorf("error starting transaction: %v", err)
            }

            // Split migration into separate statements
            statements := strings.Split(m.SQL, ";")
            for _, stmt := range statements {
                // Skip empty statements
                stmt = strings.TrimSpace(stmt)
                if stmt == "" {
                    continue
                }
                
                // Run each statement
                if _, err := tx.Exec(stmt); err != nil {
                    tx.Rollback()
                    return fmt.Errorf("error running migration %d: %v\nStatement: %s", m.Version, err, stmt)
                }
            }

            // Record migration
            if _, err := tx.Exec(
                "INSERT INTO schema_migrations (version, description) VALUES (?, ?)",
                m.Version, m.Description,
            ); err != nil {
                tx.Rollback()
                return fmt.Errorf("error recording migration %d: %v", m.Version, err)
            }

            if err := tx.Commit(); err != nil {
                return fmt.Errorf("error committing migration %d: %v", m.Version, err)
            }

            fmt.Printf("Completed migration %d\n", m.Version)
        }
    }

    return nil
} 