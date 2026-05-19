package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB

func initDB() {
	var err error
	// สร้างไฟล์ชื่อ tu_lab.db (SQLite จะสร้างให้เองถ้ายังไม่มี)
	db, err = sql.Open("sqlite3", "./tu_lab.db")
	if err != nil {
		log.Fatal(err)
	}

	// 1. สร้างตาราง Users (คงเดิม)
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE,
		password TEXT,
		role TEXT
	);`
	_, err = db.Exec(query)
	if err != nil {
		log.Fatal(err)
	}

	// 2. [แก้ไข] เพิ่มตาราง rooms เพื่อเก็บรายชื่อห้องปฏิบัติการ (3 ห้อง)
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS rooms (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		capacity INTEGER NOT NULL
	);`)
	if err != nil {
		log.Fatal(err)
	}

	// 3. [แก้ไข] ปรับปรุงตาราง reservations เพิ่มคอลัมน์ seats_count และ note
	// เพื่อรองรับกฎการเช็คความจุห้อง 100 เครื่อง และเหตุผลของอาจารย์
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS reservations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		room_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		time_slot TEXT NOT NULL,
		date TEXT NOT NULL,
		seats_count INTEGER NOT NULL,  -- เพิ่ม: จำนวนเครื่องที่จอง (นศ. = 1, อาจารย์ = 100)
		status TEXT DEFAULT 'pending',
		note TEXT,                     -- เพิ่ม: หมายเหตุการจองสำหรับอาจารย์
		FOREIGN KEY(room_id) REFERENCES rooms(id)
	);`)
	if err != nil {
		log.Fatal(err)
	}

	// 4. [เพิ่มใหม่] สร้างตาราง pinned_rooms สำหรับฟิเจอร์ปักหมุดและแจ้งเตือนทาง Email
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS pinned_rooms (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		room_id TEXT NOT NULL,
		FOREIGN KEY(room_id) REFERENCES rooms(id)
	);`)
	if err != nil {
		log.Fatal(err)
	}

	// 5. [เพิ่มใหม่] เรียกฟังก์ชันใส่ข้อมูลเริ่มต้นสำหรับทดสอบระบบ
	setupInitialData()
}

func setupMockUser(username, password, role string) {
	// เข้ารหัส Password ก่อนเก็บลง DB (Best Practice)
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	_, _ = db.Exec("INSERT OR IGNORE INTO users (username, password, role) VALUES (?, ?, ?)",
		username, string(hashedPassword), role)
}

// [เพิ่มใหม่] ฟังก์ชันสำหรับเตรียมข้อมูลห้องเรียนและรายการจองจำลอง
func setupInitialData() {
	// สร้างสิทธิ์ User ตัวอย่างสำหรับทดสอบแต่ละ Role (ตาม Scope ข้อ 5)
	setupMockUser("student001", "password123", "student")
	setupMockUser("teacher001", "password123", "teacher")
	setupMockUser("staff001", "password123", "staff")

	// เพิ่มรายชื่อห้องปฏิบัติการคอมพิวเตอร์ 3 ห้อง (ตาม Scope ข้อ 4)
	db.Exec("INSERT OR IGNORE INTO rooms (id, name, capacity) VALUES ('LAB701', 'Computer Lab 1', 100)")
	db.Exec("INSERT OR IGNORE INTO rooms (id, name, capacity) VALUES ('LAB702', 'Computer Lab 2', 100)")
	db.Exec("INSERT OR IGNORE INTO rooms (id, name, capacity) VALUES ('LAB703', 'Computer Lab 3', 100)")

	// จำลองข้อมูลการจองในอดีตหรือปัจจุบัน (ตัวอย่างวันที่ 2026-05-20) เพื่อใช้เทสระบบดึงข้อมูล
	// รายการที่ 1: นักศึกษาจองรอบแรก 1 เครื่อง (สถานะอนุมัติแล้ว -> ห้องจะเหลือ 99 เครื่อง)
	db.Exec(`INSERT OR IGNORE INTO reservations (id, user_id, room_id, time_slot, date, seats_count, status, note) 
		VALUES (1, 'student001', 'LAB701', '09:30–11:00', '2026-05-20', 1, 'approved', '')`)

	// รายการที่ 2: อาจารย์จองรอบสองทั้งห้อง 100 เครื่อง (สถานะอนุมัติแล้ว -> ห้องจะเต็ม 0 เครื่อง)
	db.Exec(`INSERT OR IGNORE INTO reservations (id, user_id, room_id, time_slot, date, seats_count, status, note) 
		VALUES (2, 'teacher001', 'LAB701', '11:00–12:30', '2026-05-20', 100, 'approved', 'สอนวิชา CS367')`)
}
