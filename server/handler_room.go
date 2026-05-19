package main

import (
	"context"
	"tu-lab-booking/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ฟังก์ชันแสดงรายการห้อง Lab ทั้งหมด (เทียบเท่า GET /api/rooms)
func (s *server) GetRooms(ctx context.Context, req *pb.Empty) (*pb.RoomList, error) {
	// 1. Query ดึงข้อมูลห้องทั้งหมดจากตาราง rooms ใน SQLite
	rows, err := db.Query("SELECT id, name, capacity FROM rooms")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ไม่สามารถดึงข้อมูลห้องได้: %v", err)
	}
	defer rows.Close()

	var rooms []*pb.Room

	// 2. วนลูปอ่านข้อมูลทีละแถวเพื่อใส่เข้าไปใน List
	for rows.Next() {
		var r pb.Room
		err := rows.Scan(&r.Id, &r.Name, &r.Capacity)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "เกิดข้อผิดพลาดในการอ่านข้อมูลห้อง: %v", err)
		}
		rooms = append(rooms, &r)
	}

	// 3. ส่งข้อมูลกลับไปในรูปแบบ RoomList
	return &pb.RoomList{
		Rooms: rooms,
	}, nil
}
