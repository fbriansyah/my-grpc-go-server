package database

import (
	"database/sql"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DatabaseAdapter struct {
	db *gorm.DB
}

func NewDatabaseAdapter(conn *sql.DB) (*DatabaseAdapter, error) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: conn,
	}), &gorm.Config{})

	if err != nil {
		return nil, fmt.Errorf("[gorm] Can't connect to database")
	}

	return &DatabaseAdapter{db: db}, nil
}
