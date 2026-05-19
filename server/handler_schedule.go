package main

import (
	"context"
	"fmt"
	"tu-lab-booking/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *server) GetRoomSchedule(ctx context.Context, req *pb.GetRoomScheduleRequest) (*pb.RoomScheduleResponse, error) {
	allSlots := []string{"08:00-09:30", "09:30-11:00", "11:00-12:30", "13:00-14:30", "14:30-16:00", "16:00-17:30"}

	rows, err := db.Query(
		"SELECT time_slot, user_id FROM reservations WHERE room_id = ? AND date = ? AND status != 'cancelled'",
		req.RoomId, req.Date,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "query error: %v", err)
	}
	defer rows.Close()

	bookedMap := make(map[string]string)
	for rows.Next() {
		var timeSlot, userID string
		if err := rows.Scan(&timeSlot, &userID); err != nil {
			return nil, status.Errorf(codes.Internal, "scan error: %v", err)
		}
		bookedMap[timeSlot] = userID
	}

	var slots []*pb.ScheduleSlot
	for _, slot := range allSlots {
		if userID, booked := bookedMap[slot]; booked {
			slots = append(slots, &pb.ScheduleSlot{TimeSlot: slot, Status: "booked", BookedBy: userID})
		} else {
			slots = append(slots, &pb.ScheduleSlot{TimeSlot: slot, Status: "available", BookedBy: ""})
		}
	}

	return &pb.RoomScheduleResponse{
		RoomId: req.RoomId,
		Date:   req.Date,
		Slots:  slots,
	}, nil
}

func (s *server) GetMyReservations(ctx context.Context, req *pb.Empty) (*pb.MyReservationsResponse, error) {
	userID, ok := ctx.Value("username").(string)
	if !ok || userID == "" {
		return nil, status.Error(codes.Unauthenticated, "ไม่พบข้อมูลผู้ใช้จาก token")
	}

	rows, err := db.Query(`
		SELECT id, room_id, date, time_slot, status
		FROM reservations
		WHERE user_id = ?
		ORDER BY date DESC, time_slot ASC
	`, userID)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "ไม่สามารถดึงประวัติการจองได้: %v", err)
	}
	defer rows.Close()

	var reservations []*pb.MyReservation
	for rows.Next() {
		var id int
		var roomID, date, timeSlot, reservationStatus string

		err := rows.Scan(&id, &roomID, &date, &timeSlot, &reservationStatus)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "อ่านข้อมูลประวัติการจองผิดพลาด: %v", err)
		}

		reservations = append(reservations, &pb.MyReservation{
			ReservationId: fmt.Sprintf("%d", id),
			RoomId:        roomID,
			Date:          date,
			TimeSlot:      timeSlot,
			Status:        reservationStatus,
		})
	}

	return &pb.MyReservationsResponse{
		Reservations: reservations,
	}, nil
}
