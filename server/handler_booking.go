package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"
	"tu-lab-booking/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *server) CreateReservation(ctx context.Context, req *pb.ReservationRequest) (*pb.ReservationResponse, error) {
	username, okUsername := ctx.Value("username").(string)
	role, okRole := ctx.Value("user_role").(string)
	if !okUsername || username == "" || !okRole || role == "" {
		return nil, status.Errorf(codes.Unauthenticated, "ข้อมูลผู้ใช้งานจาก Token ไม่ถูกต้อง")
	}

	if req.RoomId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "กรุณาระบุ room_id")
	}
	if req.Date == "" || req.TimeSlot == "" {
		return nil, status.Errorf(codes.InvalidArgument, "กรุณาระบุ date และ time_slot")
	}

	parsedDate, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "รูปแบบวันที่ไม่ถูกต้อง ควรเป็น YYYY-MM-DD")
	}

	now := time.Now().Truncate(24 * time.Hour)
	daysAhead := parsedDate.Sub(now).Hours() / 24
	if daysAhead < 0 || daysAhead > 7 {
		return nil, status.Errorf(codes.InvalidArgument, "สามารถจองล่วงหน้าได้ไม่เกิน 7 วัน")
	}

	var seatsToBook int
	var note string

	if role == "student" {
		seatsToBook = 1
	} else if role == "teacher" {
		seatsToBook = 100
		note = "อาจารย์จองเพื่อการเรียนการสอน/สอบ"
	} else {
		return nil, status.Errorf(codes.PermissionDenied, "สิทธิ์ของคุณไม่สามารถทำการจองได้")
	}

	var currentBookedSeats sql.NullInt64
	err = db.QueryRow(
		`SELECT SUM(seats_count) FROM reservations WHERE room_id = ? AND date = ? AND time_slot = ? AND status != 'rejected' AND status != 'cancelled'`,
		req.RoomId, req.Date, req.TimeSlot,
	).Scan(&currentBookedSeats)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "database error: %v", err)
	}

	if currentBookedSeats.Int64+int64(seatsToBook) > 100 {
		return nil, status.Errorf(codes.ResourceExhausted, "ขออภัย เครื่องคอมพิวเตอร์ในรอบเวลานี้เต็มแล้ว (คงเหลือ %d เครื่อง)", 100-currentBookedSeats.Int64)
	}

	result, err := db.Exec(
		`INSERT INTO reservations (room_id, user_id, date, time_slot, seats_count, status, note) VALUES (?, ?, ?, ?, ?, 'pending', ?)`,
		req.RoomId, username, req.Date, req.TimeSlot, seatsToBook, note,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "insert error: %v", err)
	}

	lastInsertID, _ := result.LastInsertId()

	return &pb.ReservationResponse{
		ReservationId: fmt.Sprintf("%d", lastInsertID),
		Status:        "pending",
		Message:       "ส่งคำขอจองสำเร็จ อยู่ระหว่างรอเจ้าหน้าที่อนุมัติ",
	}, nil
}

func (s *server) CancelReservation(ctx context.Context, req *pb.CancelReservationRequest) (*pb.CancelReservationResponse, error) {
	username, ok := ctx.Value("username").(string)
	if !ok || username == "" {
		return nil, status.Errorf(codes.Unauthenticated, "ข้อมูลผู้ใช้งานจาก Token ไม่ถูกต้อง")
	}

	if req.ReservationId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "กรุณาระบุ ReservationId")
	}

	var owner string
	var currentStatus string

	err := db.QueryRow("SELECT user_id, status FROM reservations WHERE id = ?", req.ReservationId).Scan(&owner, &currentStatus)
	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.NotFound, "ไม่พบรายการจองนี้ในระบบ")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Database error: %v", err)
	}

	if owner != username {
		return nil, status.Errorf(codes.PermissionDenied, "คุณไม่มีสิทธิ์ยกเลิกรายการจองของผู้อื่น")
	}
	if currentStatus == "cancelled" {
		return nil, status.Errorf(codes.FailedPrecondition, "รายการจองนี้ถูกยกเลิกไปก่อนหน้านี้แล้ว")
	}

	_, err = db.Exec("UPDATE reservations SET status = 'cancelled' WHERE id = ?", req.ReservationId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ไม่สามารถยกเลิกการจองได้เนื่องจากระบบภายในมีปัญหา")
	}

	return &pb.CancelReservationResponse{
		Success: true,
		Message: "ยกเลิกรายการจองของคุณเรียบร้อยแล้ว",
	}, nil
}