package db

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"fmt"
	"time"
)

// Database wraps the sql.DB connection
type Database struct {
	sqlDB *sql.DB
}

// DB returns the underlying *sql.DB instance
func (db *Database) DB() *sql.DB {
	return db.sqlDB
}

// Connect creates a new database connection
func Connect(dsn string) (*Database, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &Database{sqlDB: db}, nil
}

func (db *Database) Close() error {
	return db.sqlDB.Close()
}

// Add database methods
func (db *Database) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.sqlDB.Exec(query, args...)
}

func (db *Database) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.sqlDB.Query(query, args...)
}

func (db *Database) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.sqlDB.QueryRow(query, args...)
}

func (db *Database) BackupTables() error {
	// Get current timestamp
	timestamp := time.Now().Format("20060102_150405")
	
	// Backup each table
	tables := []string{"visit", "conversion", "campaign"}
	for _, table := range tables {
		backupTable := fmt.Sprintf("%s_backup_%s", table, timestamp)
		query := fmt.Sprintf("CREATE TABLE %s AS SELECT * FROM %s", backupTable, table)
		
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to backup table %s: %v", table, err)
		}
	}
	return nil
} 