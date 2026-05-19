package main

import (
	"context"
	"database/sql"
	"testing"

	"tu-lab-booking/pb"

	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// myreservation test unit
func setupTestDB(t *testing.T) {
	t.Helper()

	oldDB := db

	testDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("เปิด test database ไม่ได้: %v", err)
	}

	_, err = testDB.Exec(`
		CREATE TABLE reservations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			room_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			time_slot TEXT NOT NULL,
			date TEXT NOT NULL,
			status TEXT DEFAULT 'pending'
		);
	`)
	if err != nil {
		t.Fatalf("สร้างตาราง reservations ไม่ได้: %v", err)
	}

	db = testDB

	t.Cleanup(func() {
		testDB.Close()
		db = oldDB
	})
}

func TestGetMyReservationsWithoutUsername(t *testing.T) {
	setupTestDB(t)

	s := &server{}

	resp, err := s.GetMyReservations(context.Background(), &pb.Empty{})

	if err == nil {
		t.Fatal("ต้องได้ error เมื่อไม่มี username ใน context")
	}

	if resp != nil {
		t.Fatalf("response ควรเป็น nil แต่ได้: %v", resp)
	}

	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("ควรได้ Unauthenticated แต่ได้: %v", status.Code(err))
	}
}

func TestGetMyReservationsEmptyList(t *testing.T) {
	setupTestDB(t)

	s := &server{}

	ctx := context.WithValue(context.Background(), "username", "student001")

	resp, err := s.GetMyReservations(ctx, &pb.Empty{})
	if err != nil {
		t.Fatalf("ไม่ควรเกิด error: %v", err)
	}

	if resp == nil {
		t.Fatal("response ไม่ควรเป็น nil")
	}

	if len(resp.Reservations) != 0 {
		t.Fatalf("ควรไม่มีรายการจอง แต่ได้ %d รายการ", len(resp.Reservations))
	}
}

func TestGetMyReservationsOnlyOwnReservations(t *testing.T) {
	setupTestDB(t)

	_, err := db.Exec(`
		INSERT INTO reservations (room_id, user_id, time_slot, date, status)
		VALUES
		('ROOM001', 'student001', '09:30-11:00', '2026-05-08', 'pending'),
		('ROOM002', 'student001', '13:00-14:30', '2026-05-09', 'approved'),
		('ROOM003', 'teacher001', '14:30-16:00', '2026-05-10', 'pending');
	`)
	if err != nil {
		t.Fatalf("เพิ่มข้อมูลทดสอบไม่ได้: %v", err)
	}

	s := &server{}

	ctx := context.WithValue(context.Background(), "username", "student001")

	resp, err := s.GetMyReservations(ctx, &pb.Empty{})
	if err != nil {
		t.Fatalf("ไม่ควรเกิด error: %v", err)
	}

	if resp == nil {
		t.Fatal("response ไม่ควรเป็น nil")
	}

	if len(resp.Reservations) != 2 {
		t.Fatalf("ควรได้รายการจองของ student001 จำนวน 2 รายการ แต่ได้ %d รายการ", len(resp.Reservations))
	}

	for _, reservation := range resp.Reservations {
		if reservation.RoomId == "ROOM003" {
			t.Fatal("ไม่ควรดึงรายการจองของ teacher001 กลับมา")
		}
	}
}

func TestGetMyReservationsQueryError(t *testing.T) {
	setupTestDB(t)

	// ลบตาราง reservations เพื่อให้ db.Query error
	_, err := db.Exec(`DROP TABLE reservations;`)
	if err != nil {
		t.Fatalf("ลบตาราง reservations ไม่ได้: %v", err)
	}

	s := &server{}
	ctx := context.WithValue(context.Background(), "username", "student001")

	resp, err := s.GetMyReservations(ctx, &pb.Empty{})

	if err == nil {
		t.Fatal("ควรเกิด error เมื่อ query ตาราง reservations ไม่ได้")
	}

	if resp != nil {
		t.Fatalf("response ควรเป็น nil แต่ได้: %v", resp)
	}

	if status.Code(err) != codes.Internal {
		t.Fatalf("ควรได้ Internal แต่ได้: %v", status.Code(err))
	}
}

func TestGetMyReservationsScanError(t *testing.T) {
	oldDB := db

	testDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("เปิด test database ไม่ได้: %v", err)
	}

	_, err = testDB.Exec(`
		CREATE TABLE reservations (
			id TEXT PRIMARY KEY,
			room_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			time_slot TEXT NOT NULL,
			date TEXT NOT NULL,
			status TEXT DEFAULT 'pending'
		);
	`)
	if err != nil {
		t.Fatalf("สร้างตาราง reservations ไม่ได้: %v", err)
	}

	_, err = testDB.Exec(`
		INSERT INTO reservations (id, room_id, user_id, time_slot, date, status)
		VALUES ('invalid-id', 'ROOM001', 'student001', '09:30-11:00', '2026-05-08', 'pending');
	`)
	if err != nil {
		t.Fatalf("เพิ่มข้อมูลทดสอบไม่ได้: %v", err)
	}

	db = testDB

	t.Cleanup(func() {
		testDB.Close()
		db = oldDB
	})

	s := &server{}
	ctx := context.WithValue(context.Background(), "username", "student001")

	resp, err := s.GetMyReservations(ctx, &pb.Empty{})

	if err == nil {
		t.Fatal("ควรเกิด error เมื่อ scan id จาก text เป็น int ไม่ได้")
	}

	if resp != nil {
		t.Fatalf("response ควรเป็น nil แต่ได้: %v", resp)
	}

	if status.Code(err) != codes.Internal {
		t.Fatalf("ควรได้ Internal แต่ได้: %v", status.Code(err))
	}
}
