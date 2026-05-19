package main

import (
	"context"
	"database/sql"
	"tu-lab-booking/pb"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *server) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	var dbPassword string
	var role string

	err := db.QueryRow("SELECT password, role FROM users WHERE username = ?", req.Username).Scan(&dbPassword, &role)
	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.Unauthenticated, "ไม่พบชื่อผู้ใช้งาน")
	}

	err = bcrypt.CompareHashAndPassword([]byte(dbPassword), []byte(req.Password))
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "รหัสผ่านไม่ถูกต้อง")
	}

	token, err := generateToken(req.Username, role)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ไม่สามารถสร้าง Token ได้")
	}

	return &pb.LoginResponse{
		AccessToken: token,
		Role:        role,
	}, nil
}
