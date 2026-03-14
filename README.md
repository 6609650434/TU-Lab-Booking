## Feature ของระบบ
1. เข้าสู่ระบบ - POST /api/auth/login รับหน้าที่ ทีม
2. แสดงรายการห้อง Lab ทั้งหมด - GET  /api/rooms รับหน้าที่ ทีม
3. ดูตารางการใช้งานของห้องตามวันและรอบเวลา - GET /api/rooms/{id}/schedule รับหน้าที่ พงศ์
4. ส่งคำขอจองห้องหรือเครื่อง - POST /api/reservations รับหน้าที่ นอธ
5. ดูประวัติการจองของตนเอง - GET /api/reservations/my รับหน้าที่ โดนัด
6. ยกเลิกการจองของตนเอง - DELETE /api/reservations/{id} รับหน้าที่ บาส