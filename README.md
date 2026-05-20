# TU Lab Booking — ระบบจองห้องปฏิบัติการคอมพิวเตอร์

ระบบสำหรับนักศึกษาและอาจารย์จองห้องปฏิบัติการคอมพิวเตอร์ของสาขาวิชาคอมพิวเตอร์ผ่านเว็บไซต์ โดยสามารถตรวจสอบตารางการใช้งานแบบ Real-time เลือกรอบเวลาที่ต้องการ และส่งคำขอจองผ่านระบบได้โดยตรง

---

## Feature ของระบบ
1. เข้าสู่ระบบ - POST /api/auth/login รับหน้าที่ ทีม
2. แสดงรายการห้อง Lab ทั้งหมด - GET  /api/rooms รับหน้าที่ ทีม
3. ดูตารางการใช้งานของห้องตามวันและรอบเวลา - GET /api/rooms/{id}/schedule รับหน้าที่ พงศ์
4. ส่งคำขอจองห้องหรือเครื่อง - POST /api/reservations รับหน้าที่ นอธ
5. ดูประวัติการจองของตนเอง - GET /api/reservations/my รับหน้าที่ โดนัด
6. ยกเลิกการจองของตนเอง - DELETE /api/reservations/{id} รับหน้าที่ บาส

# TU Lab Booking System

## System Overview

TU Lab Booking System เป็นระบบจองห้องปฏิบัติการคอมพิวเตอร์ผ่านเว็บไซต์ สำหรับนักศึกษา อาจารย์ และเจ้าหน้าที่ภายในสาขาวิชาคอมพิวเตอร์ โดยพัฒนาขึ้นในรูปแบบ RESTful API เพื่อช่วยให้ผู้ใช้งานสามารถตรวจสอบตารางการใช้งานห้องแบบ Real-time เลือกรอบเวลาที่ต้องการ และส่งคำขอจองผ่านระบบได้โดยตรง

ระบบมีการกำหนดสิทธิ์การใช้งานตามบทบาทของผู้ใช้ (Role-based Access Control) เพื่อควบคุมเงื่อนไขการจองให้เหมาะสมกับผู้ใช้งานแต่ละประเภท และช่วยลดปัญหาการจองซ้ำซ้อนหรือความผิดพลาดในการจัดการตารางห้องปฏิบัติการ

---

## Problem Solving

ระบบถูกพัฒนาขึ้นเพื่อแก้ปัญหาดังต่อไปนี้:

- ลดขั้นตอนการติดต่อเจ้าหน้าที่
- ลดความซ้ำซ้อนในการจอง
- แสดงตารางการใช้งานแบบ Real-time
- ลดภาระของเจ้าหน้าที่ในการจัดการตาราง

---

## System Scope

- แสดงรายการห้องปฏิบัติการทั้งหมด (3 ห้อง)
- แสดงตารางการใช้งานแบบ Real-time
- จำกัดสิทธิ์การจองตามบทบาทของผู้ใช้
- แสดงรายละเอียดการจองแตกต่างกันตาม Role
- จองล่วงหน้าได้ไม่เกิน 7 วัน
- เจ้าหน้าที่สามารถอนุมัติหรือปฏิเสธคำขอได้
- รองรับการปักหมุดห้องที่สนใจและแจ้งเตือนทาง Email เมื่อห้องว่าง

---

## User Roles

### Student
- จองได้ 1 เครื่อง
- จองได้สูงสุด 1 ชั่วโมง 30 นาที
- จองล่วงหน้าได้ไม่เกิน 7 วัน

### Lecturer
- จองได้ทั้งห้อง (100 เครื่อง)
- จองได้สูงสุด 3 ชั่วโมง
- ต้องระบุรายละเอียดกิจกรรมหรือรายวิชา

### Staff
- อนุมัติหรือปฏิเสธคำขอจอง
- ดูข้อมูลการจองทั้งหมด
- จัดการข้อมูลห้องปฏิบัติการ

---

## Features

| Feature | API Endpoint | Description |
|---|---|---|
| Login | `POST /api/auth/login` | เข้าสู่ระบบและตรวจสอบสิทธิ์ผู้ใช้ |
| Get Rooms | `GET /api/rooms` | ดึงข้อมูลห้องปฏิบัติการทั้งหมด |
| Room Schedule | `GET /api/rooms/{id}/schedule` | ดูตารางการใช้งานของห้อง |
| Create Reservation | `POST /api/reservations` | ส่งคำขอจองห้องหรือเครื่อง |
| My Reservations | `GET /api/reservations/my` | ดูประวัติการจองของตนเอง |
| Cancel Reservation | `DELETE /api/reservations/{id}` | ยกเลิกการจองของตนเอง |

---

## Technologies Used

- RESTful API
- JWT Authentication
- Database
- Docker
- GitHub Workflow

---

## System Workflow

ผู้ใช้งานต้องเข้าสู่ระบบก่อนใช้งานฟังก์ชันหลักของระบบ หลังจากเข้าสู่ระบบแล้ว ผู้ใช้สามารถดูรายการห้องปฏิบัติการ ตรวจสอบตารางการใช้งาน ส่งคำขอจอง ดูประวัติการจอง และยกเลิกการจองของตนเองได้

ระบบใช้ JWT Authentication เพื่อยืนยันตัวตนของผู้ใช้งานก่อนเข้าถึง API ที่เกี่ยวข้องกับข้อมูลการจอง โดยผู้ใช้สามารถจัดการเฉพาะข้อมูลของตนเองเท่านั้น
---

## ผลการทดสอบ API ผ่าน Postman (API Testing Results)

ส่วนนี้แสดงหลักฐานการทดสอบระบบผ่านโปรแกรม Postman (gRPC) ทั้งในกรณีที่ทำงานสำเร็จตามเงื่อนไข (Success Cases) และกรณีที่ระบบตรวจดักจับข้อผิดพลาดตามเงื่อนไขความปลอดภัยและสิทธิ์ (Error & Edge Cases)

### 1. ระบบยืนยันตัวตน (Authentication Service)
* **เข้าสู่ระบบสำเร็จ (Success):**
  ![Login](POSTMANTEST_Result/Login.PNG)
* **กรณีรหัสผ่านไม่ถูกต้อง (Wrong Password):**
  ![Login Wrong Password](POSTMANTEST_Result/Login_Wrong_Password.PNG)
* **กรณีไม่พบชื่อผู้ใช้งานในระบบ (No Username in DB):**
  ![Login No Username](POSTMANTEST_Result/Login_No_Username_in_dB.PNG)

---

### 2. ระบบเรียกดูข้อมูลห้องปฏิบัติการ (Rooms Service)
* **เรียกดูรายชื่อห้องทั้งหมดสำเร็จ (Success):**
  ![Get Rooms](POSTMANTEST_Result/Rooms.PNG)
* **กรณีไม่ได้แนบ Access Token:**
  ![Rooms No Access Token](POSTMANTEST_Result/Rooms_No_Access_token.PNG)
* **กรณีแนบ Access Token ไม่ถูกต้องหรือหมดอายุ:**
  ![Rooms Wrong Access Token](POSTMANTEST_Result/Rooms_Wrong_access_token.PNG)

---

### 3. ระบบตารางเวลาห้องปฏิบัติการ (Room Schedule Service)
* **เรียกดูตารางเวลาและความว่างสำเร็จ (Success):**
  ![Get Room Schedule](POSTMANTEST_Result/GetRoomSchedule.PNG)
* **กรณีไม่ได้แนบ Access Token:**
  ![Get Room Schedule No Token](POSTMANTEST_Result/GetRoomSchedule_No_Access_Token.PNG)
* **กรณีแนบ Access Token ไม่ถูกต้อง:**
  ![Get Room Schedule Wrong Token](POSTMANTEST_Result/GetRoomSchedule_Wrong_Access_Token.PNG)

---

### 4. ระบบส่งคำขอจองห้องปฏิบัติการ (Create Reservation Service)
* **ส่งคำขอจองสำเร็จตามเงื่อนไข (Success):**
  ![Create Reservation Success](POSTMANTEST_Result/CreateReservation.PNG)
* **กรณีจองล่วงหน้าเกิน 7 วัน (ดักจับเงื่อนไขเวลา):**
  ![Create Reservation More than 7 Days](POSTMANTEST_Result/CreateReservation_Morethan7days.PNG)
* **รอบเวลาเต็ม ความจุเครื่องคอมพิวเตอร์ครบ 100 เครื่อง (Capacity Full):**
  ![Create Reservation Full](POSTMANTEST_Result/CreateReservation_Full.PNG)
* **กรณีไม่ได้แนบ Access Token:**
  ![Create Reservation No Token](POSTMANTEST_Result/CreateReservation_No_Access_Token.PNG)
* **กรณีแนบ Access Token ไม่ถูกต้อง:**
  ![Create Reservation Wrong Token](POSTMANTEST_Result/CreateReservation_Wrong_Access_Token.PNG)

---

### 5. ระบบดูประวัติการจองของตนเอง (Get My Reservations Service)
* **เรียกดูประวัติส่วนตัวสำเร็จ (Success):**
  ![Get My Reservations Success](POSTMANTEST_Result/GetMyReservations.PNG)
* **กรณีไม่ได้แนบ Access Token:**
  ![Get My Reservations No Token](POSTMANTEST_Result/GetMyReservations_No_Access_Token.PNG)
* **กรณีแนบ Access Token ไม่ถูกต้อง:**
  ![Get My Reservations Wrong Token](POSTMANTEST_Result/GetMyReservations_Wrong_Access_Token.PNG)

---

### 6. ระบบยกเลิกการจองของตนเอง (Cancel Reservation Service)
* **ยกเลิกรายการจองและอัปเดตสถานะสำเร็จ (Success):**
  ![Cancel Reservation Success](POSTMANTEST_Result/CancelReservation.PNG)
* **กรณีพยายามยกเลิกรายการจองที่เคยยกเลิกไปแล้ว (Duplicate Cancel):**
  ![Cancel Reservation Duplicate](POSTMANTEST_Result/CancelReservation_Duplicate_Cancle.PNG)
* **กรณีแอบไปยกเลิกรายการจองของผู้อื่น (Unauthorized / Owner Check):**
  ![Cancel Reservation Others](POSTMANTEST_Result/CancelReservation_Cancel_Others_But_No_Access.PNG)
* **กรณีระบุรหัสรายการจองไม่ถูกต้อง (Reservation Not Found):**
  ![Cancel Reservation Not Found](POSTMANTEST_Result/CancelReservation_No_Reservation_In_DB.PNG)
* **กรณีไม่ได้แนบ Access Token:**
  ![Cancel Reservation No Token](POSTMANTEST_Result/CancelReservation_No_Access_Token.PNG)
* **กรณีแนบ Access Token ไม่ถูกต้อง:**
  ![Cancel Reservation Wrong Token](POSTMANTEST_Result/CancelReservation_Wrong_Access_Token.PNG)
