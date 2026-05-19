# TU Lab Booking — ระบบจองห้องปฏิบัติการคอมพิวเตอร์

ระบบสำหรับนักศึกษาและอาจารย์จองห้องปฏิบัติการคอมพิวเตอร์ของสาขาวิชาคอมพิวเตอร์ผ่านเว็บไซต์ โดยสามารถตรวจสอบตารางการใช้งานแบบ Real-time เลือกรอบเวลาที่ต้องการ และส่งคำขอจองผ่านระบบได้โดยตรง

---

## Feature ของระบบ

ระบบพัฒนาในรูปแบบ **gRPC** ไม่ได้ใช้ HTTP Method โดยตรง แต่เรียกผ่าน RPC Method ที่กำหนดไว้ในไฟล์ `.proto`

| Feature | gRPC Method | รับหน้าที่ |
|---|---|---|
| เข้าสู่ระบบ | `BookingService.Login` | ทีม |
| แสดงรายการห้อง Lab ทั้งหมด | `BookingService.GetRooms` | ทีม |
| ดูตารางการใช้งานของห้อง | `BookingService.GetRoomSchedule` | พงศ์ |
| ส่งคำขอจองห้องหรือเครื่อง | `BookingService.CreateReservation` | นอธ |
| ดูประวัติการจองของตนเอง | `BookingService.GetMyReservations` | โดนัด |
| ยกเลิกการจองของตนเอง | `BookingService.CancelReservation` | บาส |

---

## ผู้ใช้งานในระบบ

| Role | สิทธิ์การจอง |
|---|---|
| `student` | จองได้ 1 เครื่อง ต่อ 1 รอบเวลา จองล่วงหน้าได้ไม่เกิน 7 วัน |
| `teacher` | จองทั้งห้อง (100 เครื่อง) จองล่วงหน้าได้ไม่เกิน 7 วัน |
| `staff` | อนุมัติ/ปฏิเสธคำขอจอง ดูตารางการจองทั้งหมด |

---

## Tech Stack

- **Language:** Go
- **Protocol:** gRPC
- **Database:** SQLite
- **Authentication:** JWT (HS256)
- **Container:** Docker

---

## การติดตั้งและรันระบบ

### Prerequisites
- Go 1.25+
- GCC (สำหรับ go-sqlite3)
- Docker Desktop (ถ้าต้องการรันผ่าน Docker)

---

### วิธีที่ 1 — รันโดยตรง

**1. Clone repository**
```bash
git clone <repository-url>
cd TU-Lab-Booking
```

**2. ติดตั้ง dependencies**
```bash
go mod download
```

**3. รัน server**
```bash
go run ./server
```

Server จะรันที่ `localhost:50051`

---

### วิธีที่ 2 — รันผ่าน Docker

**1. Pull image จาก DockerHub**
```bash
docker pull puttipong6609650541/tu-lab-booking:latest
```

**2. รัน container**
```bash
docker run -p 50051:50051 puttipong6609650541/tu-lab-booking:latest
```

**หรือใช้ docker-compose**
```bash
docker-compose up
```

---

### วิธีที่ 3 — Build Docker Image เอง

```bash
docker-compose up --build
```

---

## การทดสอบระบบ

### Unit Test

```bash
cd server
go test -coverprofile="coverage.out" .
go tool cover -func "coverage.out"
```

ผลลัพธ์: **coverage 84.5%** (เกินกว่า 80% ที่กำหนด)

### API Testing (Postman)

1. เปิด Postman → **New** → เลือก **gRPC**
2. ใส่ URL: `localhost:50051`
3. Import proto file: `proto/booking.proto`
4. เลือก method ที่ต้องการทดสอบ
5. ใส่ Token ใน **Metadata** → key: `authorization`

**ข้อมูลสำหรับทดสอบ**

| Username | Password | Role |
|---|---|---|
| `student001` | `password123` | student |
| `teacher001` | `password123` | teacher |
| `staff001` | `password123` | staff |

---

## Database Schema

ระบบใช้ **SQLite** และสร้างตารางอัตโนมัติตอน server start

| ตาราง | คำอธิบาย |
|---|---|
| `users` | เก็บข้อมูลผู้ใช้งานและ role |
| `rooms` | เก็บข้อมูลห้องปฏิบัติการ 3 ห้อง (LAB701, LAB702, LAB703) |
| `reservations` | เก็บข้อมูลการจอง |
| `pinned_rooms` | เก็บข้อมูลการปักหมุดแจ้งเตือน |

---

## Docker Image

สามารถดึง image มารันได้ที่ DockerHub โดยใช้คำสั่ง: `puttipong6609650541/tu-lab-booking:latest` 

```bash
docker pull puttipong6609650541/tu-lab-booking:latest
```

---

## Git Workflow

```
main          — version สุดท้ายของโครงงาน
develop       — รวมงานจากทุก feature
feature/*     — พัฒนาแต่ละ feature
```
