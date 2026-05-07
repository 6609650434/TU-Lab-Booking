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

	// สร้างตาราง Users
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

	// สร้าง User ตัวอย่างสำหรับทดสอบ (เฉพาะถ้ายังไม่มี)
	setupMockUser("student001", "password123", "student")
}

func setupMockUser(username, password, role string) {
	// เข้ารหัส Password ก่อนเก็บลง DB (Best Practice)
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	_, _ = db.Exec("INSERT OR IGNORE INTO users (username, password, role) VALUES (?, ?, ?)",
		username, string(hashedPassword), role)
}
