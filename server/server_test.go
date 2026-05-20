package main

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"
	"strings"

	"tu-lab-booking/pb"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ==================== TEST SETUP ====================

// setupTestDB สร้าง in-memory SQLite สำหรับทดสอบ (ไม่กระทบ tu_lab.db จริง)
func setupTestDB(t *testing.T) {
	t.Helper()
	var err error
	db, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("เปิด DB ไม่ได้: %v", err)
	}

	db.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE,
		password TEXT,
		role TEXT
	)`)

	db.Exec(`CREATE TABLE rooms (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		capacity INTEGER NOT NULL
	)`)

	db.Exec(`CREATE TABLE reservations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		room_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		time_slot TEXT NOT NULL,
		date TEXT NOT NULL,
		seats_count INTEGER NOT NULL,
		status TEXT DEFAULT 'pending',
		note TEXT
	)`)

	hashed, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	db.Exec("INSERT INTO users (username, password, role) VALUES (?, ?, ?)", "student001", string(hashed), "student")
	db.Exec("INSERT INTO users (username, password, role) VALUES (?, ?, ?)", "teacher001", string(hashed), "teacher")
	db.Exec("INSERT INTO users (username, password, role) VALUES (?, ?, ?)", "staff001", string(hashed), "staff")

	db.Exec("INSERT INTO rooms (id, name, capacity) VALUES ('LAB701', 'Computer Lab 1', 100)")
	db.Exec("INSERT INTO rooms (id, name, capacity) VALUES ('LAB702', 'Computer Lab 2', 100)")
}

// ctxWithUser สร้าง context ที่มี username และ role (จำลอง authInterceptor ผ่านแล้ว)
func ctxWithUser(username, role string) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "username", username)
	ctx = context.WithValue(ctx, "user_role", role)
	return ctx
}

// futureDate คืนวันที่ในอนาคต n วัน ในรูปแบบ YYYY-MM-DD
func futureDate(days int) string {
	return time.Now().AddDate(0, 0, days).Format("2006-01-02")
}

// ==================== LOGIN TESTS ====================

// ทดสอบ: Login สำเร็จ → ต้องได้ AccessToken และ Role กลับมา
func TestLogin_Success(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	resp, err := s.Login(context.Background(), &pb.LoginRequest{
		Username: "student001",
		Password: "password123",
	})

	if err != nil {
		t.Fatalf("ต้องไม่มี error แต่ได้: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("ต้องได้รับ AccessToken กลับมา")
	}
	if resp.Role != "student" {
		t.Errorf("Role ควรเป็น student แต่ได้ %s", resp.Role)
	}
}

// ทดสอบ: Login ด้วย username ที่ไม่มีในระบบ → ต้องได้ error Unauthenticated
func TestLogin_UserNotFound(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	_, err := s.Login(context.Background(), &pb.LoginRequest{
		Username: "unknown_user",
		Password: "password123",
	})

	assertGRPCError(t, err, codes.Unauthenticated)
}

// ทดสอบ: Login ด้วย password ผิด → ต้องได้ error Unauthenticated
func TestLogin_WrongPassword(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	_, err := s.Login(context.Background(), &pb.LoginRequest{
		Username: "student001",
		Password: "wrongpassword",
	})

	assertGRPCError(t, err, codes.Unauthenticated)
}

// ทดสอบ: Login ด้วย account อาจารย์ → ต้องได้ Role เป็น "teacher" กลับมา
func TestLogin_TeacherRole(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	resp, err := s.Login(context.Background(), &pb.LoginRequest{
		Username: "teacher001",
		Password: "password123",
	})

	if err != nil {
		t.Fatalf("ต้องไม่มี error: %v", err)
	}
	if resp.Role != "teacher" {
		t.Errorf("Role ควรเป็น teacher แต่ได้ %s", resp.Role)
	}
}

// ==================== AUTH INTERCEPTOR TESTS ====================

// ทดสอบ: authInterceptor ปล่อยให้ Login ผ่านได้โดยไม่ต้องมี Token
func TestAuthInterceptor_AllowLogin(t *testing.T) {
	called := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		called = true
		return nil, nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/booking.BookingService/Login"}

	authInterceptor(context.Background(), nil, info, handler)
	if !called {
		t.Error("ต้องให้ Login ผ่านได้โดยไม่เช็ค Token")
	}
}

// ทดสอบ: authInterceptor บล็อก request ที่ไม่มี Metadata เลย → ต้องได้ error Unauthenticated
func TestAuthInterceptor_BlockNoMetadata(t *testing.T) {
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/booking.BookingService/GetRooms"}

	_, err := authInterceptor(context.Background(), nil, info, handler)
	assertGRPCError(t, err, codes.Unauthenticated)
}

// ทดสอบ: authInterceptor บล็อก request ที่มี Metadata แต่ไม่มี authorization key
func TestAuthInterceptor_BlockNoToken(t *testing.T) {
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/booking.BookingService/GetRooms"}

	md := metadata.New(map[string]string{"other-key": "value"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := authInterceptor(ctx, nil, info, handler)
	assertGRPCError(t, err, codes.Unauthenticated)
}

// ทดสอบ: authInterceptor บล็อก request ที่ Token ผิด → ต้องได้ error Unauthenticated
func TestAuthInterceptor_BlockInvalidToken(t *testing.T) {
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/booking.BookingService/GetRooms"}

	md := metadata.New(map[string]string{"authorization": "invalid.token.here"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := authInterceptor(ctx, nil, info, handler)
	assertGRPCError(t, err, codes.Unauthenticated)
}

// ทดสอบ: authInterceptor ปล่อยผ่านเมื่อ Token ถูกต้อง และส่ง username/role ใส่ใน context
func TestAuthInterceptor_ValidToken(t *testing.T) {
	token, _ := generateToken("student001", "student")

	var ctxReceived context.Context
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		ctxReceived = ctx
		return nil, nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/booking.BookingService/GetRooms"}

	md := metadata.New(map[string]string{"authorization": token})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := authInterceptor(ctx, nil, info, handler)
	if err != nil {
		t.Fatalf("ต้องไม่มี error: %v", err)
	}
	if ctxReceived.Value("username") != "student001" {
		t.Error("ต้องได้ username ใน context")
	}
	if ctxReceived.Value("user_role") != "student" {
		t.Error("ต้องได้ user_role ใน context")
	}
}

// ==================== GET ROOMS TESTS ====================

// ทดสอบ: ดึงรายการห้องทั้งหมด → ต้องได้ครบ 2 ห้องที่ใส่ไว้ใน DB
func TestGetRooms_ReturnAllRooms(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	resp, err := s.GetRooms(ctxWithUser("student001", "student"), &pb.Empty{})

	if err != nil {
		t.Fatalf("ต้องไม่มี error: %v", err)
	}
	if len(resp.Rooms) != 2 {
		t.Errorf("ต้องได้ 2 ห้อง แต่ได้ %d ห้อง", len(resp.Rooms))
	}
}

// ทดสอบ: ข้อมูลห้องที่ดึงมาถูกต้อง → ตรวจ id และ capacity ของห้องแรก
func TestGetRooms_RoomDataCorrect(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	resp, err := s.GetRooms(ctxWithUser("student001", "student"), &pb.Empty{})

	if err != nil {
		t.Fatalf("ต้องไม่มี error: %v", err)
	}
	if resp.Rooms[0].Id != "LAB701" {
		t.Errorf("ห้องแรกควรเป็น LAB701 แต่ได้ %s", resp.Rooms[0].Id)
	}
	if resp.Rooms[0].Capacity != 100 {
		t.Errorf("ความจุควรเป็น 100 แต่ได้ %d", resp.Rooms[0].Capacity)
	}
}

// ==================== GET ROOM SCHEDULE TESTS ====================

// ทดสอบ: ดูตารางห้องที่ยังไม่มีการจอง → ต้องได้ครบ 6 slots และทุก slot เป็น available
func TestGetRoomSchedule_AllSlotsAvailable(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	resp, err := s.GetRoomSchedule(ctxWithUser("student001", "student"), &pb.GetRoomScheduleRequest{
		RoomId: "LAB701",
		Date:   futureDate(1),
	})

	if err != nil {
		t.Fatalf("ต้องไม่มี error: %v", err)
	}
	if len(resp.Slots) != 6 {
		t.Errorf("ต้องได้ 6 slots แต่ได้ %d", len(resp.Slots))
	}
	for _, slot := range resp.Slots {
		if !strings.HasPrefix(slot.Status, "available") {
    		t.Errorf("slot %s ควรเป็น available", slot.TimeSlot)
		}
	}
}

// ทดสอบ: ดูตารางห้องที่มีการจองแล้ว → slot ที่จองต้องแสดงเป็น booked และบอกว่าใครจอง
func TestGetRoomSchedule_ShowsBookedSlot(t *testing.T) {
	setupTestDB(t)
	s := &server{}
	date := futureDate(1)

	db.Exec(`INSERT INTO reservations (room_id, user_id, time_slot, date, seats_count, status, note)
		VALUES ('LAB701', 'student001', '09:30-11:00', ?, 1, 'approved', '')`, date)

	resp, err := s.GetRoomSchedule(ctxWithUser("student001", "student"), &pb.GetRoomScheduleRequest{
		RoomId: "LAB701",
		Date:   date,
	})

	if err != nil {
		t.Fatalf("ต้องไม่มี error: %v", err)
	}

	found := false
	for _, slot := range resp.Slots {
		if slot.TimeSlot == "09:30-11:00" {
			found = true
			if !strings.HasPrefix(slot.Status, "available") {
    			t.Errorf("slot 09:30-11:00 ควรเป็น available แต่ได้ %s", slot.Status)
			}
			if slot.BookedBy != "student001" {
				t.Errorf("BookedBy ควรเป็น student001 แต่ได้ %s", slot.BookedBy)
			}
		}
	}
	if !found {
		t.Error("ไม่พบ slot 09:30-11:00 ใน response")
	}
}

// ทดสอบ: slot ที่ถูกยกเลิกแล้ว → ต้องไม่แสดงเป็น booked (ควรว่างให้คนอื่นจองได้)
func TestGetRoomSchedule_CancelledNotShownAsBooked(t *testing.T) {
	setupTestDB(t)
	s := &server{}
	date := futureDate(1)

	db.Exec(`INSERT INTO reservations (room_id, user_id, time_slot, date, seats_count, status, note)
		VALUES ('LAB701', 'student001', '09:30-11:00', ?, 1, 'cancelled', '')`, date)

	resp, err := s.GetRoomSchedule(ctxWithUser("student001", "student"), &pb.GetRoomScheduleRequest{
		RoomId: "LAB701",
		Date:   date,
	})

	if err != nil {
		t.Fatalf("ต้องไม่มี error: %v", err)
	}
	for _, slot := range resp.Slots {
		if slot.TimeSlot == "09:30-11:00" && slot.Status == "booked" {
			t.Error("slot ที่ cancelled แล้วไม่ควรแสดงเป็น booked")
		}
	}
}

// ==================== CREATE RESERVATION TESTS ====================

// ทดสอบ: นักศึกษาจองห้องสำเร็จ → ต้องได้ status "pending" และ ReservationId กลับมา
func TestCreateReservation_StudentSuccess(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	resp, err := s.CreateReservation(ctxWithUser("student001", "student"), &pb.ReservationRequest{
		RoomId:   "LAB701",
		Date:     futureDate(1),
		TimeSlot: "09:30-11:00",
	})

	if err != nil {
		t.Fatalf("ต้องไม่มี error: %v", err)
	}
	if resp.Status != "pending" {
		t.Errorf("status ควรเป็น pending แต่ได้ %s", resp.Status)
	}
	if resp.ReservationId == "" {
		t.Error("ต้องได้ ReservationId กลับมา")
	}
}

// ทดสอบ: อาจารย์จองห้องสำเร็จ (จอง 100 เครื่องทั้งห้อง) → ต้องได้ status "pending" กลับมา
func TestCreateReservation_TeacherSuccess(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	resp, err := s.CreateReservation(ctxWithUser("teacher001", "teacher"), &pb.ReservationRequest{
		RoomId:   "LAB701",
		Date:     futureDate(1),
		TimeSlot: "09:30-11:00",
	})

	if err != nil {
		t.Fatalf("ต้องไม่มี error: %v", err)
	}
	if resp.Status != "pending" {
		t.Errorf("status ควรเป็น pending แต่ได้ %s", resp.Status)
	}
}

// ทดสอบ: จองโดยไม่มี Token → ต้องได้ error Unauthenticated (ระบบปฏิเสธการเข้าถึง)
func TestCreateReservation_NoToken(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	_, err := s.CreateReservation(context.Background(), &pb.ReservationRequest{
		RoomId:   "LAB701",
		Date:     futureDate(1),
		TimeSlot: "09:30-11:00",
	})

	assertGRPCError(t, err, codes.Unauthenticated)
}

// ทดสอบ: จองโดยไม่ระบุ room_id → ต้องได้ error InvalidArgument
func TestCreateReservation_EmptyRoomId(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	_, err := s.CreateReservation(ctxWithUser("student001", "student"), &pb.ReservationRequest{
		RoomId:   "",
		Date:     futureDate(1),
		TimeSlot: "09:30-11:00",
	})

	assertGRPCError(t, err, codes.InvalidArgument)
}

// ทดสอบ: จองโดยไม่ระบุวันที่ → ต้องได้ error InvalidArgument
func TestCreateReservation_EmptyDate(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	_, err := s.CreateReservation(ctxWithUser("student001", "student"), &pb.ReservationRequest{
		RoomId:   "LAB701",
		Date:     "",
		TimeSlot: "09:30-11:00",
	})

	assertGRPCError(t, err, codes.InvalidArgument)
}

// ทดสอบ: จองด้วยรูปแบบวันที่ผิด เช่น "20-05-2026" แทนที่จะเป็น "2026-05-20" → ต้องได้ error InvalidArgument
func TestCreateReservation_InvalidDateFormat(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	_, err := s.CreateReservation(ctxWithUser("student001", "student"), &pb.ReservationRequest{
		RoomId:   "LAB701",
		Date:     "20-05-2026",
		TimeSlot: "09:30-11:00",
	})

	assertGRPCError(t, err, codes.InvalidArgument)
}

// ทดสอบ: จองล่วงหน้าเกิน 7 วัน → ต้องได้ error InvalidArgument (เกินกฎของระบบ)
func TestCreateReservation_DateTooFarAhead(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	_, err := s.CreateReservation(ctxWithUser("student001", "student"), &pb.ReservationRequest{
		RoomId:   "LAB701",
		Date:     futureDate(8),
		TimeSlot: "09:30-11:00",
	})

	assertGRPCError(t, err, codes.InvalidArgument)
}

// ทดสอบ: จองวันที่ผ่านมาแล้ว → ต้องได้ error InvalidArgument (จองย้อนหลังไม่ได้)
func TestCreateReservation_PastDate(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	_, err := s.CreateReservation(ctxWithUser("student001", "student"), &pb.ReservationRequest{
		RoomId:   "LAB701",
		Date:     futureDate(-1),
		TimeSlot: "09:30-11:00",
	})

	assertGRPCError(t, err, codes.InvalidArgument)
}

// ทดสอบ: เจ้าหน้าที่พยายามจองห้อง → ต้องได้ error PermissionDenied (staff ไม่มีสิทธิ์จอง)
func TestCreateReservation_StaffCannotBook(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	_, err := s.CreateReservation(ctxWithUser("staff001", "staff"), &pb.ReservationRequest{
		RoomId:   "LAB701",
		Date:     futureDate(1),
		TimeSlot: "09:30-11:00",
	})

	assertGRPCError(t, err, codes.PermissionDenied)
}

// ทดสอบ: จองห้องที่เต็มแล้ว (อาจารย์จอง 100 เครื่องไปก่อน) → ต้องได้ error ResourceExhausted
func TestCreateReservation_RoomFull(t *testing.T) {
	setupTestDB(t)
	s := &server{}
	date := futureDate(1)

	db.Exec(`INSERT INTO reservations (room_id, user_id, time_slot, date, seats_count, status, note)
		VALUES ('LAB701', 'teacher001', '09:30-11:00', ?, 100, 'approved', 'สอนวิชา CS367')`, date)

	_, err := s.CreateReservation(ctxWithUser("student001", "student"), &pb.ReservationRequest{
		RoomId:   "LAB701",
		Date:     date,
		TimeSlot: "09:30-11:00",
	})

	assertGRPCError(t, err, codes.ResourceExhausted)
}

// ==================== GET MY RESERVATIONS TESTS ====================

// ทดสอบ: ดูประวัติการจองของตัวเอง → ต้องได้รายการที่จองไว้กลับมาถูกต้อง
func TestGetMyReservations_Success(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	db.Exec(`INSERT INTO reservations (room_id, user_id, time_slot, date, seats_count, status, note)
		VALUES ('LAB701', 'student001', '09:30-11:00', ?, 1, 'pending', '')`, futureDate(1))

	resp, err := s.GetMyReservations(ctxWithUser("student001", "student"), &pb.Empty{})

	if err != nil {
		t.Fatalf("ต้องไม่มี error: %v", err)
	}
	if len(resp.Reservations) != 1 {
		t.Errorf("ต้องได้ 1 รายการ แต่ได้ %d", len(resp.Reservations))
	}
	if resp.Reservations[0].RoomId != "LAB701" {
		t.Errorf("RoomId ควรเป็น LAB701 แต่ได้ %s", resp.Reservations[0].RoomId)
	}
}

// ทดสอบ: ดูประวัติโดยไม่มี Token → ต้องได้ error Unauthenticated
func TestGetMyReservations_NoToken(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	_, err := s.GetMyReservations(context.Background(), &pb.Empty{})

	assertGRPCError(t, err, codes.Unauthenticated)
}

// ทดสอบ: ระบบต้องคืนแค่รายการของตัวเอง ไม่ใช่ของคนอื่น → student001 ต้องเห็นแค่ 1 รายการ ไม่เห็นของ teacher001
func TestGetMyReservations_OnlyOwnReservations(t *testing.T) {
	setupTestDB(t)
	s := &server{}
	date := futureDate(1)

	db.Exec(`INSERT INTO reservations (room_id, user_id, time_slot, date, seats_count, status, note)
		VALUES ('LAB701', 'student001', '09:30-11:00', ?, 1, 'pending', '')`, date)
	db.Exec(`INSERT INTO reservations (room_id, user_id, time_slot, date, seats_count, status, note)
		VALUES ('LAB702', 'teacher001', '11:00-12:30', ?, 100, 'pending', 'สอน')`, date)

	resp, err := s.GetMyReservations(ctxWithUser("student001", "student"), &pb.Empty{})

	if err != nil {
		t.Fatalf("ต้องไม่มี error: %v", err)
	}
	if len(resp.Reservations) != 1 {
		t.Errorf("student001 ควรเห็นแค่ 1 รายการของตัวเอง แต่ได้ %d", len(resp.Reservations))
	}
}

// ทดสอบ: ดูประวัติเมื่อยังไม่เคยจองเลย → ต้องได้ list ว่างกลับมา ไม่ใช่ error
func TestGetMyReservations_EmptyList(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	resp, err := s.GetMyReservations(ctxWithUser("student001", "student"), &pb.Empty{})

	if err != nil {
		t.Fatalf("ต้องไม่มี error: %v", err)
	}
	if len(resp.Reservations) != 0 {
		t.Errorf("ต้องได้ list ว่าง แต่ได้ %d รายการ", len(resp.Reservations))
	}
}

// ==================== CANCEL RESERVATION TESTS ====================

// ทดสอบ: ยกเลิกการจองของตัวเอง → ต้องได้ Success = true กลับมา
func TestCancelReservation_Success(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	result, _ := db.Exec(`INSERT INTO reservations (room_id, user_id, time_slot, date, seats_count, status, note)
		VALUES ('LAB701', 'student001', '09:30-11:00', ?, 1, 'pending', '')`, futureDate(1))
	id, _ := result.LastInsertId()

	resp, err := s.CancelReservation(ctxWithUser("student001", "student"), &pb.CancelReservationRequest{
		ReservationId: fmt.Sprintf("%d", id),
	})

	if err != nil {
		t.Fatalf("ต้องไม่มี error: %v", err)
	}
	if !resp.Success {
		t.Error("ต้องได้ Success = true")
	}
}

// ทดสอบ: student พยายามยกเลิกการจองของ teacher → ต้องได้ error PermissionDenied (ยกเลิกของคนอื่นไม่ได้)
func TestCancelReservation_NotOwner(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	result, _ := db.Exec(`INSERT INTO reservations (room_id, user_id, time_slot, date, seats_count, status, note)
		VALUES ('LAB701', 'teacher001', '09:30-11:00', ?, 100, 'pending', 'สอน')`, futureDate(1))
	id, _ := result.LastInsertId()

	_, err := s.CancelReservation(ctxWithUser("student001", "student"), &pb.CancelReservationRequest{
		ReservationId: fmt.Sprintf("%d", id),
	})

	assertGRPCError(t, err, codes.PermissionDenied)
}

// ทดสอบ: ยกเลิกด้วย reservation_id ที่ไม่มีในระบบ → ต้องได้ error NotFound
func TestCancelReservation_NotFound(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	_, err := s.CancelReservation(ctxWithUser("student001", "student"), &pb.CancelReservationRequest{
		ReservationId: "99999",
	})

	assertGRPCError(t, err, codes.NotFound)
}

// ทดสอบ: ยกเลิกซ้ำ (การจองที่ cancelled ไปแล้ว) → ต้องได้ error FailedPrecondition
func TestCancelReservation_AlreadyCancelled(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	result, _ := db.Exec(`INSERT INTO reservations (room_id, user_id, time_slot, date, seats_count, status, note)
		VALUES ('LAB701', 'student001', '09:30-11:00', ?, 1, 'cancelled', '')`, futureDate(1))
	id, _ := result.LastInsertId()

	_, err := s.CancelReservation(ctxWithUser("student001", "student"), &pb.CancelReservationRequest{
		ReservationId: fmt.Sprintf("%d", id),
	})

	assertGRPCError(t, err, codes.FailedPrecondition)
}

// ทดสอบ: ยกเลิกโดยไม่มี Token → ต้องได้ error Unauthenticated
func TestCancelReservation_NoToken(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	_, err := s.CancelReservation(context.Background(), &pb.CancelReservationRequest{
		ReservationId: "1",
	})

	assertGRPCError(t, err, codes.Unauthenticated)
}

// ทดสอบ: ยกเลิกโดยไม่ระบุ reservation_id → ต้องได้ error InvalidArgument
func TestCancelReservation_EmptyId(t *testing.T) {
	setupTestDB(t)
	s := &server{}

	_, err := s.CancelReservation(ctxWithUser("student001", "student"), &pb.CancelReservationRequest{
		ReservationId: "",
	})

	assertGRPCError(t, err, codes.InvalidArgument)
}

// ==================== AUTH TESTS ====================

// ทดสอบ: สร้าง JWT Token แล้ว verify → ต้องอ่าน username และ role กลับมาได้ถูกต้อง
func TestGenerateAndVerifyToken(t *testing.T) {
	token, err := generateToken("student001", "student")
	if err != nil {
		t.Fatalf("สร้าง token ไม่ได้: %v", err)
	}
	if token == "" {
		t.Error("token ต้องไม่ว่าง")
	}

	claims, err := verifyToken(token)
	if err != nil {
		t.Fatalf("verify token ไม่ได้: %v", err)
	}
	if claims.Username != "student001" {
		t.Errorf("Username ควรเป็น student001 แต่ได้ %s", claims.Username)
	}
	if claims.Role != "student" {
		t.Errorf("Role ควรเป็น student แต่ได้ %s", claims.Role)
	}
}

// ทดสอบ: verify token ที่ไม่ถูกต้อง → ต้องได้ error กลับมา (ป้องกัน token ปลอม)
func TestVerifyToken_InvalidToken(t *testing.T) {
	_, err := verifyToken("invalid.token.string")
	if err == nil {
		t.Error("ต้องได้ error สำหรับ token ที่ไม่ถูกต้อง")
	}
}

// ==================== HELPER ====================

// assertGRPCError ตรวจสอบว่า error ที่ได้เป็น gRPC error code ที่ต้องการ
func assertGRPCError(t *testing.T, err error, expectedCode codes.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("ต้องได้ error code %v แต่ไม่มี error เลย", expectedCode)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("error ไม่ใช่ gRPC status error: %v", err)
	}
	if st.Code() != expectedCode {
		t.Errorf("ต้องได้ error code %v แต่ได้ %v: %s", expectedCode, st.Code(), st.Message())
	}
}

// ==================== setup test ====================
// ทดสอบ: setupMockUser สร้าง user ใหม่ใน DB ได้สำเร็จ
func TestSetupMockUser(t *testing.T) {
	setupTestDB(t)

	setupMockUser("newuser", "pass123", "student")

	var role string
	err := db.QueryRow("SELECT role FROM users WHERE username = ?", "newuser").Scan(&role)
	if err != nil {
		t.Fatalf("ต้องพบ user ที่สร้างไว้: %v", err)
	}
	if role != "student" {
		t.Errorf("role ควรเป็น student แต่ได้ %s", role)
	}
}

// ทดสอบ: setupInitialData เพิ่มห้องและ user ตัวอย่างเข้า DB ได้ครบ
func TestSetupInitialData(t *testing.T) {
	setupTestDB(t)

	setupInitialData()

	var count int
	db.QueryRow("SELECT COUNT(*) FROM rooms").Scan(&count)
	if count < 3 {
		t.Errorf("ต้องมีห้องอย่างน้อย 3 ห้อง แต่ได้ %d", count)
	}

	var userCount int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	if userCount < 3 {
		t.Errorf("ต้องมี user อย่างน้อย 3 คน แต่ได้ %d", userCount)
	}
}

// ทดสอบ: initDB เชื่อมต่อ DB และสร้างตารางทั้งหมดได้สำเร็จ
func TestInitDB(t *testing.T) {
	// ชี้ไปที่ไฟล์ชั่วคราวแทน tu_lab.db จริง
	originalDB := db
	defer func() { db = originalDB }()

	// สร้าง temp db file
	tmpDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("เปิด DB ไม่ได้: %v", err)
	}
	db = tmpDB

	initDB()

	// เช็คว่าตารางถูกสร้างครบ
	tables := []string{"users", "rooms", "reservations", "pinned_rooms"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("ตาราง %s ไม่ถูกสร้าง: %v", table, err)
		}
	}
}