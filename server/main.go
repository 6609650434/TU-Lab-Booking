package main

import (
	"log"
	"net"

	"tu-lab-booking/pb" // ตรวจสอบว่าชื่อ module ตรงกับใน go.mod ของคุณ

	"google.golang.org/grpc"
)

// 1. สร้าง Struct สำหรับ Server ให้ไฟล์อื่นเรียกใช้
type server struct {
	pb.UnimplementedBookingServiceServer
}

func main() {
	// เรียกใช้ฟังก์ชันเชื่อมต่อ Database จากไฟล์ database.go
	initDB()
	log.Println("เชื่อมต่อ SQLite สำเร็จ...")

	// เริ่มต้น gRPC Server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// ใส่ Interceptor เข้าไปใน Server
	s := grpc.NewServer(
		grpc.UnaryInterceptor(authInterceptor),
	)

	pb.RegisterBookingServiceServer(s, &server{})

	log.Println("gRPC Server รันอยู่ที่พอร์ต :50051...")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
