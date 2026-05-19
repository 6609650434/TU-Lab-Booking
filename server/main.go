package main

import (
	"context"
	"log"
	"net"
	"time"

	"database/sql"
	"tu-lab-booking/pb" // ตรวจสอบว่าชื่อ module ตรงกับใน go.mod ของคุณ

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

func (s *server) CancelReservation(ctx context.Context, req *pb.CancelReservationRequest) (*pb.CancelReservationResponse, error) {
	// 1. ดึง username จาก JWT Token อย่างปลอดภัย ป้องกันแอป Panic
	username, ok := ctx.Value("username").(string)
	if !ok || username == "" {
		return nil, status.Errorf(codes.Unauthenticated, "ข้อมูลผู้ใช้งานจาก Token ไม่ถูกต้อง")
	}

	// ตรวจสอบว่าส่ง ID รายการที่จะยกเลิกมาหรือไม่
	if req.ReservationId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "กรุณาระบุ ReservationId")
	}

	var owner string
	var currentStatus string

	// 2. ดึงข้อมูล user_id และ status ปัจจุบันมาตรวจสอบก่อน
	err := db.QueryRow(
		"SELECT user_id, status FROM reservations WHERE id = ?",
		req.ReservationId,
	).Scan(&owner, &currentStatus)

	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.NotFound, "ไม่พบรายการจองนี้ในระบบ")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Database error: %v", err)
	}

	// 3. ตรวจสอบสิทธิ์: ต้องเป็นเจ้าของรายการจองนี้เท่านั้นถึงจะยกเลิกได้
	if owner != username {
		return nil, status.Errorf(codes.PermissionDenied, "คุณไม่มีสิทธิ์ยกเลิกรายการจองของผู้อื่น")
	}

	// 4. (เสริม) เช็คว่ารายการนี้เคยถูกยกเลิก หรือเจ้าหน้าที่ปฏิเสธไปแล้วหรือยัง
	if currentStatus == "cancelled" {
		return nil, status.Errorf(codes.FailedPrecondition, "รายการจองนี้ถูกยกเลิกไปก่อนหน้านี้แล้ว")
	}

	// 5. [แก้ไข] เปลี่ยนจาก DELETE เป็นการ UPDATE สถานะแทนเพื่อไม่ให้ประวัติหาย
	_, err = db.Exec(
		"UPDATE reservations SET status = 'cancelled' WHERE id = ?",
		req.ReservationId,
	)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "ไม่สามารถยกเลิกการจองได้เนื่องจากระบบภายในมีปัญหา")
	}

	return &pb.CancelReservationResponse{
		Success: true,
		Message: "ยกเลิกรายการจองของคุณเรียบร้อยแล้ว",
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
