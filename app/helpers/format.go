package helpers

import (
	"fmt"
	"time"
)

// Tanggal memformat waktu menjadi string dd-mm-yyyy hh:mm:ss.
func Tanggal(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("02-01-2006 15:04:05")
}

// FormatWeight memformat angka berat ke satuan gram dengan satu angka desimal.
func FormatWeight(w float64) string {
	return fmt.Sprintf("%.1f g", w)
}

