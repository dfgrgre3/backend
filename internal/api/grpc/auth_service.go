package grpc

import (
	"context"
	"errors"
	"log"
	"strings"
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
	tokenService *services.TokenService
	userRepo     *repository.UserRepository
}

func NewAuthServiceServer() *AuthServiceServer {
	userRepo := repository.NewUserRepository(db.DB)
	return &AuthServiceServer{
		tokenService: &services.TokenService{},
		userRepo:     userRepo,
	}
}

func (s *AuthServiceServer) Login(ctx context.Context, req *thanawyv1.LoginRequest) (*thanawyv1.LoginResponse, error) {
	return nil, errors.New("legacy authentication is retired. Please use Clerk for authentication")
}

func (s *AuthServiceServer) Register(ctx context.Context, req *thanawyv1.RegisterRequest) (*thanawyv1.RegisterResponse, error) {
	return nil, errors.New("legacy registration is retired. Please use Clerk for registration")
}

func (s *AuthServiceServer) GetProfile(ctx context.Context, req *thanawyv1.GetProfileRequest) (*thanawyv1.GetProfileResponse, error) {
	userID, ok := ctx.Value(middleware.UserContextKey).(string)
	if !ok || userID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		// User not found in DB — attempt to provision from Clerk.
		// The middleware may have set a clerkUserId in context when the Clerk ID
		// differs from the resolved DB UUID.
		clerkID, _ := ctx.Value(middleware.ClerkIDContextKey).(string)
		if clerkID == "" {
			// If no explicit clerkID, the userID itself may be a Clerk-format ID.
			if strings.HasPrefix(userID, "user_") {
				clerkID = userID
			}
		}

		if clerkID != "" && strings.HasPrefix(clerkID, "user_") {
			log.Printf("[GetProfile] User %s not found in DB, provisioning from Clerk ID: %s", userID, clerkID)
			provisioned, provErr := services.ProvisionUserFromClerk(clerkID)
			if provErr != nil {
				log.Printf("[GetProfile] Provisioning failed for Clerk ID %s: %v", clerkID, provErr)
				return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
			}
			user = provisioned
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
