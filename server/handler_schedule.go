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

	// Query รวม seats_count และดึง user_id คนล่าสุดที่จองใน slot นั้น
	rows, err := db.Query(
		`SELECT time_slot, COALESCE(SUM(seats_count), 0) as total_seats,
		 (SELECT user_id FROM reservations r2 
		  WHERE r2.room_id = r1.room_id AND r2.date = r1.date AND r2.time_slot = r1.time_slot 
		  AND r2.status != 'cancelled' AND r2.status != 'rejected'
		  ORDER BY r2.id DESC LIMIT 1) as last_user
		 FROM reservations r1
		 WHERE room_id = ? AND date = ? AND status != 'cancelled' AND status != 'rejected'
		 GROUP BY time_slot`,
		req.RoomId, req.Date,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "query error: %v", err)
	}
	defer rows.Close()

	// เก็บ total_seats และ last_user ของแต่ละ slot
	type slotData struct {
		totalSeats int64
		lastUser   string
	}
	bookedMap := make(map[string]slotData)
	for rows.Next() {
		var timeSlot, lastUser string
		var totalSeats int64
		if err := rows.Scan(&timeSlot, &totalSeats, &lastUser); err != nil {
			return nil, status.Errorf(codes.Internal, "scan error: %v", err)
		}
		bookedMap[timeSlot] = slotData{totalSeats: totalSeats, lastUser: lastUser}
	}

	var slots []*pb.ScheduleSlot
	for _, slot := range allSlots {
		data := bookedMap[slot]
		remaining := int64(100) - data.totalSeats

		var slotStatus string
		if remaining <= 0 {
			slotStatus = "booked (0/100)"
		} else {
			slotStatus = fmt.Sprintf("available (%d/100)", remaining)
		}

		slots = append(slots, &pb.ScheduleSlot{
			TimeSlot: slot,
			Status:   slotStatus,
			BookedBy: data.lastUser,
		})
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