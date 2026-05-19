package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"database/sql"
	"tu-lab-booking/pb" // ตรวจสอบว่าชื่อ module ตรงกับใน go.mod

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// 1. สร้าง Struct สำหรับ Server
type server struct {
	pb.UnimplementedBookingServiceServer
}

var jwtSecret = []byte("tu-lab-booking-secret-key-2026")

type MyCustomClaims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func generateToken(username, role string) (string, error) {
	// 1. ตั้งค่าข้อมูลใน Token (Claims)
	claims := MyCustomClaims{
		username,
		role,
		jwt.RegisteredClaims{
			// กำหนดวันหมดอายุ (เช่น 24 ชั่วโมง)
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// 2. สร้าง Token ด้วยอัลกอริทึม HS256
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 3. เซ็นชื่อด้วย Secret Key ของเรา
	return token.SignedString(jwtSecret)
}

func verifyToken(tokenString string) (*MyCustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &MyCustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*MyCustomClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrSignatureInvalid
}

func authInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// 1. ยกเว้นการเช็ค Token สำหรับ Method "Login" (เพราะต้อง Login ก่อนถึงจะมี Token)
	if info.FullMethod == "/booking.BookingService/Login" {
		return handler(ctx, req)
	}

	// 2. ดึง Metadata (เหมือน Header ใน HTTP) ออกมา
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "ไม่มี Metadata ส่งมาด้วย")
	}

	// 3. หาค่า 'authorization' (ปกติจะส่งมาในรูปแบบ "Bearer <token>")
	values := md.Get("authorization")
	if len(values) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "ไม่พบ Token ใน Metadata")
	}

	// 4. ตรวจสอบความถูกต้องของ Token
	tokenString := values[0]
	claims, err := verifyToken(tokenString)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "Token ไม่ถูกต้องหรือหมดอายุ: %v", err)
	}

	// 5. ส่งข้อมูล User (เช่น Role) เข้าไปใน Context เพื่อให้ฟังก์ชันอื่นๆ ใช้งานต่อได้
	newCtx := context.WithValue(ctx, "user_role", claims.Role)
	newCtx = context.WithValue(newCtx, "username", claims.Username)

	return handler(newCtx, req)
}

// 2. ฟังก์ชัน Login ที่ดึงข้อมูลจากฐานข้อมูล SQLite
func (s *server) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	var dbPassword string
	var role string

	// 1. ตรวจสอบ User ใน SQLite
	err := db.QueryRow("SELECT password, role FROM users WHERE username = ?", req.Username).Scan(&dbPassword, &role)
	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.Unauthenticated, "ไม่พบชื่อผู้ใช้งาน")
	}

	// 2. ตรวจสอบ Password
	err = bcrypt.CompareHashAndPassword([]byte(dbPassword), []byte(req.Password))
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "รหัสผ่านไม่ถูกต้อง")
	}

	// 3. สร้าง JWT Token ของจริง
	token, err := generateToken(req.Username, role)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ไม่สามารถสร้าง Token ได้")
	}

	return &pb.LoginResponse{
		AccessToken: token,
		Role:        role,
	}, nil
}

// ฟังก์ชันดึงข้อมูลของห้องมา
func (s *server) GetRoomSchedule(ctx context.Context, req *pb.GetRoomScheduleRequest) (*pb.RoomScheduleResponse, error) {
	// กำหนด time slots ทั้งหมดที่มีในระบบ
	allSlots := []string{
		"08:00-09:30",
		"09:30-11:00",
		"11:00-12:30",
		"13:00-14:30",
		"14:30-16:00",
		"16:00-17:30",
	}

	// Query ดึง reservations ของห้องนี้ในวันที่ระบุ
	rows, err := db.Query(
		"SELECT time_slot, user_id FROM reservations WHERE room_id = ? AND date = ?",
		req.RoomId, req.Date,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "query error: %v", err)
	}
	defer rows.Close()

	// สร้าง map เพื่อ lookup ว่า slot ไหนถูกจองแล้ว
	bookedMap := make(map[string]string) // time_slot -> user_id
	for rows.Next() {
		var timeSlot, userID string
		if err := rows.Scan(&timeSlot, &userID); err != nil {
			return nil, status.Errorf(codes.Internal, "scan error: %v", err)
		}
		bookedMap[timeSlot] = userID
	}

	// สร้าง response slots
	var slots []*pb.ScheduleSlot
	for _, slot := range allSlots {
		if userID, booked := bookedMap[slot]; booked {
			slots = append(slots, &pb.ScheduleSlot{
				TimeSlot: slot,
				Status:   "booked",
				BookedBy: userID,
			})
		} else {
			slots = append(slots, &pb.ScheduleSlot{
				TimeSlot: slot,
				Status:   "available",
				BookedBy: "",
			})
		}
	}

	return &pb.RoomScheduleResponse{
		RoomId: req.RoomId,
		Date:   req.Date,
		Slots:  slots,
	}, nil
}

// ฟังก์ชันสำหรับจองห้อง
func (s *server) CreateReservation(ctx context.Context, req *pb.ReservationRequest) (*pb.ReservationResponse, error) {
	// 1. ดึง username และ role จาก JWT Token
	username, okUsername := ctx.Value("username").(string)
	role, okRole := ctx.Value("user_role").(string)
	if !okUsername || username == "" || !okRole || role == "" {
		return nil, status.Errorf(codes.Unauthenticated, "ข้อมูลผู้ใช้งานจาก Token ไม่ถูกต้อง")
	}

	// 2. ตรวจสอบข้อมูลพื้นฐาน (แก้ไขบั๊กสตริง RoomId)
	if req.RoomId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "กรุณาระบุ room_id")
	}
	if req.Date == "" || req.TimeSlot == "" {
		return nil, status.Errorf(codes.InvalidArgument, "กรุณาระบุ date และ time_slot")
	}

	// 3. ตรวจสอบเงื่อนไขเวลา: จองล่วงหน้าไม่เกิน 7 วัน
	parsedDate, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "รูปแบบวันที่ไม่ถูกต้อง ควรเป็น YYYY-MM-DD")
	}

	// หาส่วนต่างของวัน (นับจากเวลาปัจจุบัน)
	now := time.Now().Truncate(24 * time.Hour)
	daysAhead := parsedDate.Sub(now).Hours() / 24
	if daysAhead < 0 || daysAhead > 7 {
		return nil, status.Errorf(codes.InvalidArgument, "สามารถจองล่วงหน้าได้ไม่เกิน 7 วัน")
	}

	// 4. กำหนดจำนวนเครื่องที่จะจองตามสิทธิ์ (Role-based Rules)
	var seatsToBook int
	var note string

	if role == "student" {
		seatsToBook = 1 // นักศึกษาจองได้ทีละ 1 เครื่อง
	} else if role == "teacher" {
		seatsToBook = 100 // อาจารย์จองทั้งห้อง (100 เครื่อง)
		// ในระบบจริง ควรส่งค่า note เพิ่มเข้ามาผ่าน proto ด้วยครับ แต่ตอนนี้ขอดึงค่าเบื้องต้นไว้ก่อน
		note = "อาจารย์จองเพื่อการเรียนการสอน/สอบ"
	} else {
		return nil, status.Errorf(codes.PermissionDenied, "สิทธิ์ของคุณไม่สามารถทำการจองได้")
	}

	// 5. ตรวจสอบจำนวนเครื่องที่ถูกจองไปแล้วในฐานข้อมูล (Capacity Check)
	var currentBookedSeats sql.NullInt64
	err = db.QueryRow(
		`SELECT SUM(seats_count) 
		 FROM reservations 
		 WHERE room_id = ? AND date = ? AND time_slot = ? AND status != 'rejected'`,
		req.RoomId, req.Date, req.TimeSlot,
	).Scan(&currentBookedSeats)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "database error: %v", err)
	}

	// ถ้าผลรวมเครื่องบวกกับที่จะจองใหม่ เกินความจุสูงสุด (100 เครื่อง)
	if currentBookedSeats.Int64+int64(seatsToBook) > 100 {
		return nil, status.Errorf(codes.ResourceExhausted, "ขออภัย เครื่องคอมพิวเตอร์ในรอบเวลานี้เต็มแล้ว (คงเหลือ %d เครื่อง)", 100-currentBookedSeats.Int64)
	}

	// 6. บันทึกข้อมูลลง database พร้อมคอลัมน์ใหม่
	result, err := db.Exec(
		`INSERT INTO reservations (room_id, user_id, date, time_slot, seats_count, status, note)
		 VALUES (?, ?, ?, ?, ?, 'pending', ?)`,
		req.RoomId, username, req.Date, req.TimeSlot, seatsToBook, note,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "insert error: %v", err)
	}

	// ดึง ID ล่าสุดที่เพิ่ง Insert สำเร็จเพื่อส่งกลับไปให้หน้าบ้าน
	lastInsertID, _ := result.LastInsertId()

	return &pb.ReservationResponse{
		ReservationId: fmt.Sprintf("%d", lastInsertID),
		Status:        "pending",
		Message:       "ส่งคำขอจองสำเร็จ อยู่ระหว่างรอเจ้าหน้าที่อนุมัติ",
	}, nil
}

// ฟังก์ชันดูประวัติการจองของตนเอง
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
		var roomID string
		var date string
		var timeSlot string
		var reservationStatus string

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

func main() {
	// --- ส่วนที่เพิ่มเข้ามา ---
	// เรียกใช้ฟังก์ชันเชื่อมต่อ Database จากไฟล์ database.go
	initDB()
	log.Println("เชื่อมต่อ SQLite สำเร็จ...")
	// -----------------------

	// เริ่มต้น gRPC Server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(authInterceptor),
	)
	pb.RegisterBookingServiceServer(s, &server{})

	log.Println("gRPC Server รันอยู่ที่พอร์ต :50051...")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
