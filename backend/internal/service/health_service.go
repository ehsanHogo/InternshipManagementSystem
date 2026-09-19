package service

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type HealthService struct {
	db *gorm.DB
}

func NewHealthService(db *gorm.DB) *HealthService {
	return &HealthService{db: db}
}

func (service *HealthService) Check(ctx context.Context) error {
	sqlDB, err := service.db.DB()
	if err != nil {
		return fmt.Errorf("access database connection: %w", err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	return nil
}
