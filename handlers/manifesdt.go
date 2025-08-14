package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"my-fiber-app/db" // Asumsi ini adalah package Anda untuk koneksi DB
)

const (
	cacheDir        = "./cache_manifes"
	cacheTTL        = 30 * time.Minute
	cleanupInterval = 1 * time.Minute
)

var (
	cacheMutex sync.Mutex
)

// init dijalankan sekali saat package diimpor
func init() {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Fatalf("Failed to create cache directory %s: %v", cacheDir, err)
	}
	go startCacheCleanup()
}

// DataTableResponse tetap didefinisikan untuk keperluan cache internal,
// tetapi tidak akan langsung dikirim jika Anda hanya ingin array data.
type DataTableResponse struct {
	Draw            int             `json:"draw"`
	RecordsTotal    int             `json:"recordsTotal"`
	RecordsFiltered int             `json:"recordsFiltered"`
	Data            []ManifesRecord `json:"data"`
	SearchValue     string          `json:"searchValue,omitempty"`
	ExecutedQuery   string          `json:"executedQuery,omitempty"`
}

type ManifesRecord struct {
	Dept         string       `json:"dept"`
	Manifes      int          `json:"manifes"`
	SJ           int          `json:"sj"`
	ShipDate     string       `json:"ship_date"`
	NoPol        string       `json:"no_pol"`
	POCustomer   string       `json:"po_customer"`
	Cardname     string       `json:"cardname"`
	ShipTo       string       `json:"ship_to"`
	Driver       string       `json:"driver"`
	GRStatusME   string       `json:"gr_status_me"`
	Reason       sql.NullString `json:"reason"`
	Penerima     sql.NullString `json:"penerima"`
	FotoBukti    sql.NullString `json:"foto_bukti"`
	ImgSignature sql.NullString `json:"img_signature"`
	Created      sql.NullTime   `json:"created"`
}

// AjaxManifesHandler sekarang akan mengelola caching dan hanya mengembalikan array data
func AjaxManifesHandler(c *fiber.Ctx) error {
	// --- PERBAIKAN 1: Deklarasikan variabel di awal fungsi ---
	// Pindahkan deklarasi dan inisialisasi draw, start, length ke sini
	draw, _ := strconv.Atoi(c.Query("draw"))
	start, _ := strconv.Atoi(c.Query("start"))
	length, _ := strconv.Atoi(c.Query("length"))

	searchValue := c.Query("search[value]")
	if searchValue == "" {
		searchValue = c.Query("search") // Fallback jika hanya 'search' yang ada
	}
	orderColumnIndexStr := c.Query("order[0][column]")
	orderDir := c.Query("order[0][dir]")
	dateRange := c.Query("dateRange")

	// Default values jika konversi gagal atau 0
	if length == 0 {
		length = 10 // Default DataTables length
	}

	sanitizedSearchValue := sanitizeFilename(searchValue)
	todayDate := time.Now().Format("20060102")

	cacheFilename := fmt.Sprintf("manifest_%s_search_%s_start_%s_len_%s_ordercol_%s_ordir_%s.json",
		todayDate, sanitizedSearchValue, c.Query("start"), c.Query("length"), sanitizeFilename(orderColumnIndexStr), sanitizeFilename(orderDir))

	cacheFilePath := filepath.Join(cacheDir, cacheFilename)

	// ============ LOGIKA CACHE ============
	cacheMutex.Lock() // Kunci mutex sebelum mengakses file cache
	defer cacheMutex.Unlock() // Pastikan mutex dilepaskan setelah selesai

	// 1. Coba baca dari cache
	if data, err := readCacheFile(cacheFilePath); err == nil {
		log.Printf("Cache hit for %s", cacheFilePath)
		var cachedResponse DataTableResponse
		if unmarshalErr := json.Unmarshal(data, &cachedResponse); unmarshalErr == nil {
			return c.JSON(cachedResponse.Data) // Mengembalikan hanya data
		} else {
			log.Printf("Error unmarshalling cached data: %v. Re-querying.", unmarshalErr)
		}
	} else {
		log.Printf("Cache miss or error for %s: %v. Re-querying.", cacheFilePath, err)
	}

	// 2. Jika cache miss atau error, lakukan query ke database
	orderColumn := " created"
	actualOrderDir := "DESC"
	if orderColumnIndexStr != "" {
		colIndex, err := strconv.Atoi(orderColumnIndexStr)
		if err == nil && colIndex >= 0 && colIndex < len(columnMap) && columnMap[colIndex] != "" {
			orderColumn = columnMap[colIndex]
		}
	}
	if strings.ToLower(orderDir) == "asc" {
		actualOrderDir = "ASC"
	} else {
		actualOrderDir = "DESC"
	}

	dateWhereClause := ""
	if dateRange != "" {
		log.Printf("Date range received: %s", dateRange)
	} else {
		if searchValue != "" {
			dateWhereClause = " DD.[DOCDATE] BETWEEN DATEADD(DAY, -100, GETDATE()) AND GETDATE() "
		} else {
			dateWhereClause = " DD.[DOCDATE] BETWEEN DATEADD(DAY, -30, GETDATE()) AND GETDATE() "
		}
	}

	// Pastikan getManifesDatatableDirect menerima argumen yang benar
	dataResponse, executedQuery, err := getManifesDatatableDirect(db.GetDB(), start, length, searchValue, orderColumn, actualOrderDir, dateWhereClause, draw)
	if err != nil {
		log.Printf("Error getting manifest data: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to retrieve data")
	}

	dataResponse.SearchValue = searchValue
	dataResponse.ExecutedQuery = executedQuery

	// 3. Tulis hasil query ke cache (tetap simpan format lengkap untuk konsistensi)
	if err := writeCacheFile(cacheFilePath, dataResponse); err != nil {
		log.Printf("Error writing cache file %s: %v", cacheFilePath, err)
	}

	// Kembalikan hanya bagian Data dari response
	return c.JSON(dataResponse.Data)
}

// Fungsi pembantu untuk membaca file cache
func readCacheFile(filePath string) ([]byte, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("cache file stat error: %w", err)
	}

	// Cek apakah cache masih valid (belum kadaluwarsa)
	if time.Since(fileInfo.ModTime()) > cacheTTL {
		return nil, fmt.Errorf("cache expired")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}
	return data, nil
}

// Fungsi pembantu untuk menulis data ke file cache
func writeCacheFile(filePath string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data to JSON: %w", err)
	}

	// Tulis ke file sementara dulu, lalu rename untuk atomic write
	tempFilePath := filePath + ".tmp"
	if err := os.WriteFile(tempFilePath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write temporary cache file: %w", err)
	}

	if err := os.Rename(tempFilePath, filePath); err != nil {
		return fmt.Errorf("failed to rename temporary cache file: %w", err)
	}

	return nil
}


// startCacheCleanup adalah goroutine yang berjalan di latar belakang untuk membersihkan cache
func startCacheCleanup() {
	ticker := time.NewTicker(cleanupInterval) // Cek setiap 1 menit
	defer ticker.Stop()

	for range ticker.C {
		cacheMutex.Lock()
		// Pastikan mutex selalu dilepaskan di akhir setiap siklus pembersihan
		// untuk mencegah kebuntuan.
		defer cacheMutex.Unlock()

		files, err := os.ReadDir(cacheDir)
		if err != nil {
			log.Printf("Error reading cache directory for cleanup: %v", err)
			continue
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			filePath := filepath.Join(cacheDir, file.Name())
			fileInfo, err := os.Stat(filePath)
			if err != nil {
				log.Printf("Error statting cache file %s during cleanup: %v", filePath, err)
				continue
			}

			// Hapus file jika sudah lebih tua dari cacheTTL
			if time.Since(fileInfo.ModTime()) > cacheTTL {
				if err := os.Remove(filePath); err != nil {
					log.Printf("Error removing expired cache file %s: %v", filePath, err)
				} else {
					log.Printf("Removed expired cache file: %s", filePath)
				}
			}
		}
	}
}

var columnMap = []string{
	"",
	"dept",
	"manifes",
	"sj",
	"ship_date",
	"no_pol",
	"po_customer",
	"cardname",
	"ship_to",
	"driver",
	"gr_status_me",
	"reason",
	"",
	"foto_bukti",
	"img_signature",
	"created",
}

func getManifesDatatableDirect(database *sql.DB, start, length int, searchValue, orderColumn, orderDir, dateWhereClause string, draw int) (*DataTableResponse, string, error) {
	if length <= 0 {
		return &DataTableResponse{
			Draw:            draw,
			RecordsTotal:    0,
			RecordsFiltered: 0,
			Data:            []ManifesRecord{},
		}, "No query executed due to zero or negative length", nil
	}

	baseQuery := `
		SELECT DISTINCT
			BP.U_IDU_DEPARTMENT AS dept,
			t6.MANIFEST# AS manifes,
			dd.Docnum AS sj,
			CONVERT(VARCHAR, T6.SHIP_DATE, 23) AS ship_date,
			T6.U_IDU_NoPol AS no_pol,
			dd.numatcard AS po_customer,
			dd.cardname,
			dd.ShipToCode AS ship_to,
			T6.DRIVER AS driver,
			CASE WHEN mm.[status] IS NULL THEN '' COLLATE SQL_Latin1_General_CP850_CI_AS ELSE ms.[status] END AS gr_status_me,
			mr.reason,
			mm.penerima,
			mm.foto_bukti,
			mm.img_signature,
			mm.created
		FROM
			[pksrv-sap].[pandurasa_live].dbo.ODLN DD
		INNER JOIN
			[pksrv-sap].[pandurasa_live].dbo.DLN1 D WITH (NOLOCK) ON DD.DocEntry = D.DocEntry AND D.BaseType = 17 AND DD.CANCELED = 'N'
		INNER JOIN
			[pksrv-sap].[pandurasa_live].dbo.OQUT q WITH (NOLOCK) ON dd.CardCode = q.CardCode AND dd.NumAtCard = q.NumAtCard
		INNER JOIN
			[pksrv-sap].[pandurasa_live].dbo.OITM IT WITH (NOLOCK) ON D.ITEMCODE = IT.ITEMCODE AND IT.validfor = 'Y'
		INNER JOIN
			[pksrv-sap].[pandurasa_live].dbo.oitb tb WITH (NOLOCK) ON it.ItmsGrpCod = tb.ItmsGrpCod
		INNER JOIN
			[pksrv-sap].[pandurasa_live].dbo.OCRD BP WITH (NOLOCK) ON BP.CARDCODE = DD.CARDCODE
		LEFT JOIN (
			SELECT
				t2.DOCNUM AS MANIFEST#,
				t2.U_IDU_TANGGAL AS SHIP_DATE,
				t2.U_IDU_NAMASUPIR AS DRIVER,
				t2.U_IDU_NoPol,
				U_IDU_Kode_Rute,
				t1.u_idu_nomordo AS SJ#,
				TOTAL_SHIP
			FROM
				[pksrv-sap].[PANDURASA_LIVE].[dbo].[@idu_d_manifest] t0 WITH (NOLOCK)
			INNER JOIN
				[pksrv-sap].[PANDURASA_LIVE].[dbo].[@idu_h_manifest] t2 WITH (NOLOCK) ON t0.DocEntry = t2.DocEntry
			INNER JOIN (
				SELECT
					U_IDU_NomorDO,
					COUNT(U_IDU_NomorDO) AS TOTAL_SHIP
				FROM
					[pksrv-sap].[PANDURASA_LIVE].[dbo].[@idu_d_manifest] t01 WITH (NOLOCK)
				WHERE
					t01.u_idu_nomordo IS NOT NULL AND t01.U_IDU_DocEntry IS NOT NULL
				GROUP BY
					t01.U_IDU_NomorDO
			) t1 ON t0.U_IDU_NomorDO = t1.U_IDU_NomorDO AND t1.u_idu_nomordo IS NOT NULL
		) T6 ON DD.DOCNUM = T6.SJ#
		LEFT JOIN
			[pksrv-sap].[pandurasa_live].dbo.OCRG QP WITH (NOLOCK) ON QP.GroupCode = BP.GroupCode
		INNER JOIN
			[pksrv-sap].[pandurasa_live].dbo.crd1 BP1 WITH (NOLOCK) ON BP.CardCode = BP1.CardCode AND DD.ShipToCode = BP1.[Address] AND BP1.AdresType = 'S'
		LEFT JOIN
			[pksrv-sap].[pandurasa_live].dbo.OPRC QP1 WITH (NOLOCK) ON IT.U_IDU_CC_BRAND = QP1.PrcCode
		LEFT JOIN
			[pksrv-sap].[pandurasa_live].[dbo].[@IDU_NOMOR_POLISI] T7 WITH (NOLOCK) ON T6.U_IDU_NoPol = T7.Code
		LEFT JOIN
			[pksrv-sap].[pk_express].[dbo].me_manifest mm ON dd.Docnum = mm.sj AND t6.MANIFEST# = mm.no_manifes
		LEFT JOIN
			[pksrv-sap].[pk_express].[dbo].master_status ms ON mm.[status] = ms.id
		LEFT JOIN
			[pksrv-sap].[pk_express].[dbo].master_reason mr ON mr.id = mm.reason
		WHERE
			T6.MANIFEST# IS NOT NULL AND T6.U_IDU_NoPol IS NOT NULL
	`

	if dateWhereClause != "" {
		baseQuery += " AND " + dateWhereClause
	}

	filteredQuery := baseQuery
	if searchValue != "" {
		if intVal, err := strconv.Atoi(searchValue); err == nil {
			filteredQuery += fmt.Sprintf(" AND (T6.MANIFEST# = %d OR dd.Docnum = %d)", intVal, intVal)
		} else {
			var searchConditions []string
			searchColumns := []string{
				"CAST(t6.MANIFEST# AS VARCHAR)",
				"dd.Docnum",
				"dd.cardname",
				"T6.DRIVER",
				"T6.U_IDU_NoPol",
				"ms.[status]",
				"mr.reason",
				"dd.ShipToCode",
			}
			escapedSearchValue := strings.ReplaceAll(searchValue, "[", "[[]")
			escapedSearchValue = strings.ReplaceAll(escapedSearchValue, "%", "[%]")
			escapedSearchValue = strings.ReplaceAll(escapedSearchValue, "_", "[_]")

			for _, col := range searchColumns {
				searchConditions = append(searchConditions, fmt.Sprintf("%s LIKE '%%%s%%' ESCAPE '['", col, escapedSearchValue))
			}
			filteredQuery += " AND (" + strings.Join(searchConditions, " OR ") + ")"
		}
	}

	sqlData := fmt.Sprintf(`
		SELECT * FROM (
			%s
		) AS DataWithRowNumber
		
		ORDER BY ship_date DESC, dept ASC, %s %s
		OFFSET %d ROWS
		FETCH NEXT %d ROWS ONLY;
	`, filteredQuery, orderColumn, orderDir, start, length)

	rows, err := database.Query(sqlData)
	if err != nil {
		return nil, "", fmt.Errorf("error querying data: %w", err)
	}
	defer rows.Close()

	var manifesRecords []ManifesRecord
	for rows.Next() {
		var record ManifesRecord
		err := rows.Scan(
			&record.Dept,
			&record.Manifes,
			&record.SJ,
			&record.ShipDate,
			&record.NoPol,
			&record.POCustomer,
			&record.Cardname,
			&record.ShipTo,
			&record.Driver,
			&record.GRStatusME,
			&record.Reason,
			&record.Penerima,
			&record.FotoBukti,
			&record.ImgSignature,
			&record.Created,
		)
		if err != nil {
			return nil, "", fmt.Errorf("error scanning row: %w", err)
		}
		manifesRecords = append(manifesRecords, record)
	}

	if err = rows.Err(); err != nil {
		return nil, "", fmt.Errorf("error iterating rows: %w", err)
	}

	recordsFetched := len(manifesRecords)
	return &DataTableResponse{
		Draw:            draw,
		RecordsTotal:    recordsFetched,
		RecordsFiltered: recordsFetched,
		Data:            manifesRecords,
	}, sqlData, nil
}
