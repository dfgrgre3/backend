package grpc

import (
	"context"
	"errors"
	"fmt"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/middleware"
	"thanawy-backend/internal/models"
	thanawyv1 "thanawy-backend/internal/proto/thanawy/v1"
	"thanawy-backend/internal/proto/thanawy/v1/thanawyv1connect"
	"thanawy-backend/internal/repository"
	"thanawy-backend/internal/services"

	"connectrpc.com/connect"
)

type AuthServiceServer struct {
	thanawyv1.UnimplementedAuthServiceServer
	authService  *services.AuthService
	tokenService *services.TokenService
	userRepo     *repository.UserRepository
}

func NewAuthServiceServer() *AuthServiceServer {
	userRepo := repository.NewUserRepository(db.DB)
	return &AuthServiceServer{
		authService:  services.NewAuthService(userRepo),
		tokenService: &services.TokenService{},
		userRepo:     userRepo,
	}
}

func (s *AuthServiceServer) Login(ctx context.Context, req *thanawyv1.LoginRequest) (*thanawyv1.LoginResponse, error) {
	user, err := s.authService.Login(req.Email, req.Password, "", "")
	if err != nil {
		return nil, err
	}

	token, err := s.tokenService.GenerateAccessToken(user.ID, string(user.Role))
	if err != nil {
		return nil, err
	}

	return &thanawyv1.LoginResponse{
		Success: true,
		Token:   token,
		User:    mapUserToProto(user),
	}, nil
}

func (s *AuthServiceServer) Register(ctx context.Context, req *thanawyv1.RegisterRequest) (*thanawyv1.RegisterResponse, error) {
	input := services.RegisterInput{
		Email:         req.Email,
		Username:      req.Username,
		Password:      req.Password,
		Role:          models.RoleStudent,
		Phone:         req.Phone,
		GradeLevel:    req.GradeLevel,
		EducationType: req.EducationType,
		Section:       req.Section,
	}

	user, err := s.authService.Register(input)
	if err != nil {
		return nil, err
	}

	return &thanawyv1.RegisterResponse{
		Success: true,
		User:    mapUserToProto(user),
	}, nil
}

func (s *AuthServiceServer) GetProfile(ctx context.Context, req *thanawyv1.GetProfileRequest) (*thanawyv1.GetProfileResponse, error) {
	userID, ok := ctx.Value(middleware.UserContextKey).(string)
	if !ok || userID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		email, _ := ctx.Value(middleware.EmailContextKey).(string)
		if email != "" {
			newUser := &models.User{
				ID:            userID,
				Email:         email,
				EmailVerified: true,
				Status:        models.StatusActive,
				Role:          models.RoleStudent,
				Balance:       0,
				AiCredits:     0,
				ExamCredits:   0,
				TotalXP:       0,
				Level:         1,
			}
			if createErr := db.DB.Create(newUser).Error; createErr == nil {
				user = newUser
			} else {
				if user, err = s.userRepo.FindByID(userID); err != nil {
					return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create or retrieve user profile: %w", createErr))
				}
			}
		} else {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
		}
	}

	return &thanawyv1.GetProfileResponse{
		User: mapUserToProto(user),
	}, nil
}

func (s *AuthServiceServer) Logout(ctx context.Context, req *thanawyv1.LogoutRequest) (*thanawyv1.LogoutResponse, error) {
	return &thanawyv1.LogoutResponse{
		Success: true,
		Message: "Logged out successfully",
	}, nil
}

// Connect Wrapper
type AuthConnectHandler struct {
	thanawyv1connect.UnimplementedAuthServiceHandler
	Svc *AuthServiceServer
}

func (h *AuthConnectHandler) Login(ctx context.Context, req *connect.Request[thanawyv1.LoginRequest]) (*connect.Response[thanawyv1.LoginResponse], error) {
	res, err := h.Svc.Login(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (h *AuthConnectHandler) Register(ctx context.Context, req *connect.Request[thanawyv1.RegisterRequest]) (*connect.Response[thanawyv1.RegisterResponse], error) {
	res, err := h.Svc.Register(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (h *AuthConnectHandler) GetProfile(ctx context.Context, req *connect.Request[thanawyv1.GetProfileRequest]) (*connect.Response[thanawyv1.GetProfileResponse], error) {
	res, err := h.Svc.GetProfile(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (h *AuthConnectHandler) Logout(ctx context.Context, req *connect.Request[thanawyv1.LogoutRequest]) (*connect.Response[thanawyv1.LogoutResponse], error) {
	res, err := h.Svc.Logout(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func mapUserToProto(u *models.User) *thanawyv1.User {
	if u == nil {
		return nil
	}
	return &thanawyv1.User{
		Id:       u.ID,
		Email:    u.Email,
		Username: strPtr(u.Username),
		Name:     strPtr(u.Name),
		Role:     string(u.Role),
		Avatar:   strPtr(u.Avatar),
	}
}
