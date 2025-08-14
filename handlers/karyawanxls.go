package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"sort" // <-- Tambahkan import ini

	"github.com/xuri/excelize/v2"
	"my-fiber-app/db"
)

// ProcessKaryawanExcelToJSON handles the core logic of reading Excel, saving JSON,
// and then importing that JSON data into the SQL Server database.
func ProcessKaryawanExcelToJSON() {
	excelFilePath := "\\\\192.168.60.14\\htdocs\\pk-action-plan\\hr\\karyawan.xlsx"
	outputDir := "./hr"
	outputFileName := "karyawan.json"
	outputFilePath := filepath.Join(outputDir, outputFileName)

	tableName := "dbo.master_karyawan_xls"

	log.Printf("Memulai proses konversi Excel ke JSON dan impor ke DB untuk %s", excelFilePath)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("Gagal membuat direktori '%s': %v", outputDir, err)
		return
	}

	if _, err := os.Stat(excelFilePath); os.IsNotExist(err) {
		log.Printf("File Excel tidak ditemukan: %s", err)
		return
	}

	f, err := excelize.OpenFile(excelFilePath)
	if err != nil {
		log.Printf("Gagal membuka file Excel '%s': %v", excelFilePath, err)
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("Gagal menutup file Excel: %v", err)
		}
	}()

	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		log.Println("Tidak ada sheet ditemukan di file Excel.")
		return
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		log.Printf("Gagal mendapatkan baris dari sheet '%s': %v", sheetName, err)
		return
	}

	var jsonData []map[string]interface{}
	if len(rows) == 0 {
		jsonData = []map[string]interface{}{}
		log.Println("Tidak ada data di Excel, akan membuat file JSON kosong dan membersihkan tabel DB.")
	} else {
		headers := rows[0]
		processedCleanedHeaders := make(map[string]bool)
		filteredOriginalHeaders := []string{}

		for _, originalHeader := range headers {
			cleanedHeader := cleanColumnName(originalHeader)
			if cleanedHeader != "" && !strings.EqualFold(cleanedHeader, "ID") && !processedCleanedHeaders[cleanedHeader] {
				filteredOriginalHeaders = append(filteredOriginalHeaders, originalHeader)
				processedCleanedHeaders[cleanedHeader] = true
			}
		}

		for i := 1; i < len(rows); i++ {
			row := rows[i]
			rowData := make(map[string]interface{})
			for _, originalHeader := range filteredOriginalHeaders {
				originalIndex := -1
				for hIdx, hVal := range rows[0] {
					if hVal == originalHeader {
						originalIndex = hIdx
						break
					}
				}

				if originalIndex != -1 && originalIndex < len(row) {
					rowData[originalHeader] = row[originalIndex]
				} else {
					rowData[originalHeader] = nil
				}
			}
			jsonData = append(jsonData, rowData)
		}
	}

	jsonBytes, err := json.MarshalIndent(jsonData, "", "  ")
	if err != nil {
		log.Printf("Gagal melakukan marshal JSON: %v", err)
		return
	}

	if err := os.WriteFile(outputFilePath, jsonBytes, 0644); err != nil {
		log.Printf("Gagal menulis file JSON ke '%s': %v", outputFilePath, err)
		return
	}

	log.Printf("File JSON berhasil disimpan ke: %s", outputFilePath)

	if err := importKaryawanDataToDB(jsonData, tableName); err != nil {
		log.Printf("Gagal mengimpor data ke database: %v", err)
	}
	log.Println("Proses konversi dan impor database selesai.")
}

// importKaryawanDataToDB adalah fungsi internal yang menangani impor data ke SQL Server
func importKaryawanDataToDB(karyawanData []map[string]interface{}, tableName string) error {
	log.Printf("Memulai impor %d baris data ke tabel '%s'.", len(karyawanData), tableName)

	sqlDB := db.GetDB()
	if sqlDB == nil {
		return fmt.Errorf("koneksi database tidak tersedia")
	}

	tx, err := sqlDB.Begin()
	if err != nil {
		return fmt.Errorf("gagal memulai transaksi DB: %w", err)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("Panic saat impor DB, melakukan rollback: %v", r)
			panic(r)
		}
	}()

	dropTableSQL := fmt.Sprintf("IF OBJECT_ID('%s', 'U') IS NOT NULL DROP TABLE %s;", tableName, tableName)
	log.Printf("Menjalankan SQL Drop Table: %s", dropTableSQL)
	_, err = tx.Exec(dropTableSQL)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("gagal drop tabel '%s': %w", tableName, err)
	}

	if len(karyawanData) == 0 {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("gagal commit transaksi setelah drop tabel (data kosong): %w", err)
		}
		log.Printf("Tabel '%s' berhasil di-drop karena file Excel kosong.", tableName)
		return nil
	}

	currentColumns := []string{}
	for key := range karyawanData[0] {
		currentColumns = append(currentColumns, key)
	}
	// --- PERBAIKAN DI SINI ---
	// Ganti dari:
	// strings.Slice(currentColumns, func(i, j int) bool {
	// 	return currentColumns[i] < currentColumns[j]
	// })
	// Menjadi:
	sort.Strings(currentColumns) // <-- Gunakan ini untuk Go versi lama

	createTableSQL := fmt.Sprintf("CREATE TABLE %s (id INT IDENTITY(1,1) PRIMARY KEY", tableName)
	for _, col := range currentColumns {
		cleanedColName := cleanColumnName(col)
		if cleanedColName != "" {
			createTableSQL += fmt.Sprintf(", [%s] NVARCHAR(MAX)", cleanedColName)
		}
	}
	createTableSQL += ");"
	log.Printf("Menjalankan SQL Create Table: %s", createTableSQL)
	_, err = tx.Exec(createTableSQL)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("gagal membuat tabel '%s': %w", tableName, err)
	}

	insertColumns := make([]string, len(currentColumns))
	placeholders := make([]string, len(currentColumns))
	for i, col := range currentColumns {
		insertColumns[i] = fmt.Sprintf("[%s]", cleanColumnName(col))
		placeholders[i] = fmt.Sprintf("@p%d", i+1)
	}

	insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);",
		tableName,
		strings.Join(insertColumns, ", "),
		strings.Join(placeholders, ", "),
	)
	log.Printf("Mempersiapkan statement INSERT: %s", insertSQL)

	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("gagal mempersiapkan statement INSERT: %w", err)
	}
	defer stmt.Close()

	for i, rowData := range karyawanData {
		args := make([]interface{}, len(currentColumns))
		for j, col := range currentColumns {
			args[j] = rowData[col]
		}
		_, err := stmt.Exec(args...)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("gagal insert baris %d (%+v) ke tabel: %w", i+1, rowData, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("gagal commit transaksi DB: %w", err)
	}

	log.Printf("Berhasil mengimpor %d baris ke tabel '%s'.", len(karyawanData), tableName)
	return nil
}

// cleanColumnName membersihkan nama kolom agar valid untuk SQL Server
func cleanColumnName(name string) string {
	cleaned := strings.TrimSpace(name)
	cleaned = strings.ReplaceAll(cleaned, " ", "_")
	cleaned = strings.ReplaceAll(cleaned, "/", "_")
	cleaned = strings.ReplaceAll(cleaned, "-", "_")
	var sb strings.Builder
	for _, r := range cleaned {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		}
	}
	cleaned = sb.String()

	if cleaned == "" {
		return ""
	}

	if len(cleaned) > 0 && cleaned[0] >= '0' && cleaned[0] <= '9' {
		cleaned = "Col_" + cleaned
	}

	if len(cleaned) > 128 {
		cleaned = cleaned[:128]
	}

	return cleaned
}

// StartDailyExcelToJsonUpdater schedules the Excel to JSON conversion to run daily.
func StartDailyExcelToJsonUpdater() {
	ProcessKaryawanExcelToJSON()

	now := time.Now()
	nextMidnight := now.Add(24 * time.Hour).Truncate(24 * time.Hour)
	initialDelay := nextMidnight.Sub(now)

	log.Printf("Penjadwal Excel ke JSON dan DB diaktifkan. Jalankan pertama sekarang. Selanjutnya pada %v", nextMidnight.Format("2006-01-02 15:04:05"))

	go func() {
		time.Sleep(initialDelay)
		ProcessKaryawanExcelToJSON()

		dailyTicker := time.NewTicker(24 * time.Hour)
		defer dailyTicker.Stop()
		for range dailyTicker.C {
			ProcessKaryawanExcelToJSON()
		}
	}()
}