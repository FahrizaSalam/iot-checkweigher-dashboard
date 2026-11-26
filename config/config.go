package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// LoadEnv memuat variabel konfigurasi dari file .env.
func LoadEnv() {
	if err := godotenv.Load(); err != nil {
		log.Println("File .env tidak ditemukan, memakai environment variable sistem")
	}
}

// Env mengambil nilai environment variable atau mengembalikan fallback jika kosong.
func Env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

