package service

import (
	"database/sql"
	"errors"
	"inventory/model"
	"inventory/repository"
)

type WarningService struct {
	repo *repository.WarningRepo
}

func NewWarningService(repo *repository.WarningRepo) *WarningService {
	return &WarningService{repo: repo}
}

func (s *WarningService) List(onlyUnresolved bool, page, pageSize int) ([]*model.StockWarning, int64, error) {
	return s.repo.List(onlyUnresolved, page, pageSize)
}

func (s *WarningService) Resolve(id int64) error {
	if err := s.repo.Resolve(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("预警记录不存在或已处理")
		}
		return err
	}
	return nil
}
