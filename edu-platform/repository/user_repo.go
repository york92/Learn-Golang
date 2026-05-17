package repository

import (
	"edu-platform/model"

	"gorm.io/gorm"
)

// UserRepo 用户数据访问层
type UserRepo struct {
	db *gorm.DB
}

// NewUserRepo 构造函数
func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}

// Create 创建用户
func (r *UserRepo) Create(user *model.User) error {
	return r.db.Create(user).Error
}

// FindByID 按 ID 查询用户
func (r *UserRepo) FindByID(id uint) (*model.User, error) {
	var user model.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByEmail 按邮箱查询用户
func (r *UserRepo) FindByEmail(email string) (*model.User, error) {
	var user model.User
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByUsername 按用户名查询用户
func (r *UserRepo) FindByUsername(username string) (*model.User, error) {
	var user model.User
	if err := r.db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// Update 更新用户信息
func (r *UserRepo) Update(user *model.User) error {
	return r.db.Save(user).Error
}

// Delete 软删除用户
func (r *UserRepo) Delete(id uint) error {
	return r.db.Delete(&model.User{}, id).Error
}
