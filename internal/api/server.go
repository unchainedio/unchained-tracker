package api

import (
	"unchained-tracker/internal/config"  // Updated import path
	"unchained-tracker/internal/db"      // Updated import path
	"unchained-tracker/internal/geo"     // Updated import path
)

type Server struct {
	db     *db.Database
	config *config.Config
	geo    *geo.Service
}

func NewServer(db *db.Database, config *config.Config, geo *geo.Service) *Server {
	return &Server{
		db:     db,
		config: config,
		geo:    geo,
	}
} 