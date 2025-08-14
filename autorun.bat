@echo off
REM Script untuk menjalankan aplikasi Go di C:\xampp\htdocs\fiber

REM Pindah ke direktori proyek Go Anda
cd /d C:\xampp\htdocs\fiber

REM Jalankan aplikasi Go
REM Menggunakan "start cmd /k" agar jendela CMD tetap terbuka setelah aplikasi Go berjalan
REM Ini berguna untuk melihat output atau error dari aplikasi Go
start cmd /k "go run main.go"

REM Anda juga bisa menggunakan:
REM start cmd /c "go run main.go"
REM Jika Anda ingin jendela CMD menutup secara otomatis setelah aplikasi Go berjalan (atau error)
REM Atau hanya:
REM go run main.go
REM Jika Anda tidak ingin jendela CMD terpisah dan ingin proses berjalan di background (tidak ideal untuk melihat output)

exit