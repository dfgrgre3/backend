package repositories

import (
	"sync"
	models "thanawy-backend/internal/domain/common"

	"gorm.io/gorm"
)

type SecurityLogRepository struct {
	db *gorm.DB
}

func NewSecurityLogRepository(db *gorm.DB) *SecurityLogRepository {
	return &SecurityLogRepository{db: db}
}

func (r *SecurityLogRepository) Create(log *models.SecurityLog) error {
	return r.db.Create(log).Error
}

func (r *SecurityLogRepository) FindByUserID(userID string, limit int) ([]models.SecurityLog, error) {
	var logs []models.SecurityLog
	// Note: Using user_id (snake_case) to match GORM column naming
	query := r.db.Where("user_id = ?", userID).Order("created_at desc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&logs).Error
	return logs, err
}

func (r *SecurityLogRepository) FindAll(limit, offset int) ([]models.SecurityLog, int64, error) {
	var logs []models.SecurityLog
	var count int64

	var wg sync.WaitGroup
	var errCount, errFind error

	wg.Add(2)
	go func() {
		defer wg.Done()
		// Fix: Use separate session to avoid concurrent map writes
		countRdb := r.db.Session(&gorm.Session{NewDB: true})
		errCount = countRdb.Model(&models.SecurityLog{}).Count(&count).Error
	}()

	go func() {
		defer wg.Done()
		// Fix: Use separate session to avoid concurrent map writes
		findRdb := r.db.Session(&gorm.Session{NewDB: true})
		errFind = findRdb.Order("created_at desc").Limit(limit).Offset(offset).Find(&logs).Error
	}()

	wg.Wait()

	if errCount != nil {
		return nil, 0, errCount
	}
	if errFind != nil {
		return nil, 0, errFind
	}

	return logs, count, nil
}
