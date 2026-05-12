package service

import (
	"database/sql"
	"errors"
	"fmt"
	"inventory/model"
	"inventory/repository"
)

var (
	ErrProductNotFound = errors.New("商品不存在")
	ErrInvalidParam    = errors.New("参数不合法")
)

type ProductService struct {
	repo *repository.ProductRepo
}

func NewProductService(repo *repository.ProductRepo) *ProductService {
	return &ProductService{repo: repo}
}

func (s *ProductService) Create(req model.CreateProductReq) (*model.Product, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("%w: 商品名称不能为空", ErrInvalidParam)
	}
	if req.InitialStock < 0 {
		return nil, fmt.Errorf("%w: 初始库存不能为负数", ErrInvalidParam)
	}
	if req.Price < 0 {
		return nil, fmt.Errorf("%w: 价格不能为负数", ErrInvalidParam)
	}

	p := &model.Product{
		Name:             req.Name,
		Spec:             req.Spec,
		Price:            req.Price,
		Stock:            req.InitialStock,
		WarningThreshold: req.WarningThreshold,
	}
	id, err := s.repo.Create(nil, p)
	if err != nil {
		return nil, err
	}
	p.ID = id
	return p, nil
}

func (s *ProductService) GetByID(id int64) (*model.Product, error) {
	p, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}
	return p, nil
}

func (s *ProductService) List(req model.ListProductsReq) ([]*model.Product, int64, error) {
	return s.repo.List(req)
}

func (s *ProductService) Update(id int64, req model.UpdateProductReq) (*model.Product, error) {
	if err := s.repo.UpdateInfo(id, req); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}
	return s.repo.GetByID(id)
}

func (s *ProductService) Delete(id int64) error {
	if err := s.repo.SoftDelete(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrProductNotFound
		}
		return err
	}
	return nil
}
