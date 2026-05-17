package service

import (
	"errors"

	"edu-platform/config"
	"edu-platform/model"
	"edu-platform/repository"
	"edu-platform/utils"

	"golang.org/x/crypto/bcrypt"
)

// UserService 用户业务逻辑层
type UserService struct {
	repo *repository.UserRepo
}

// NewUserService 构造函数
func NewUserService(repo *repository.UserRepo) *UserService {
	return &UserService{repo: repo}
}

// RegisterInput 注册请求参数
type RegisterInput struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role"`
}

// LoginInput 登录请求参数
type LoginInput struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse 登录/注册成功返回
type AuthResponse struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

// Register 用户注册
func (s *UserService) Register(input *RegisterInput) (*AuthResponse, error) {
	// 验证角色合法性
	role := model.UserRole(input.Role)
	if role != model.RoleStudent && role != model.RoleTeacher {
		role = model.RoleStudent // 默认学生
	}

	// 哈希密码
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("failed to hash password")
	}

	user := &model.User{
		Username: input.Username,
		Email:    input.Email,
		Password: string(hash),
		Role:     role,
		Nickname: input.Username,
	}

	if err := s.repo.Create(user); err != nil {
		return nil, errors.New("email or username already exists")
	}

	token, err := utils.GenerateToken(
		user.ID, user.Username, string(user.Role),
		config.AppConfig.JWT.Secret,
		config.AppConfig.JWT.ExpireHours,
	)
	if err != nil {
		return nil, errors.New("failed to generate token")
	}

	return &AuthResponse{Token: token, User: user}, nil
}

// Login 用户登录
func (s *UserService) Login(input *LoginInput) (*AuthResponse, error) {
	user, err := s.repo.FindByEmail(input.Email)
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		return nil, errors.New("invalid email or password")
	}

	token, err := utils.GenerateToken(
		user.ID, user.Username, string(user.Role),
		config.AppConfig.JWT.Secret,
		config.AppConfig.JWT.ExpireHours,
	)
	if err != nil {
		return nil, errors.New("failed to generate token")
	}

	return &AuthResponse{Token: token, User: user}, nil
}

// GetProfile 获取用户信息
func (s *UserService) GetProfile(userID uint) (*model.User, error) {
	return s.repo.FindByID(userID)
}
