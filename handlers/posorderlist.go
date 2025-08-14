package handlers

import (
	"database/sql"
	"log"

	"github.com/gofiber/fiber/v2"
	_ "github.com/denisenkom/go-mssqldb" 
	"my-fiber-app/db"
)
type DocOrder struct {
	DocID       string  `json:"doc_id"`
	WhsCode     string  `json:"whs_code"`
	Created     string  `json:"created"` // Menggunakan string karena format sudah ditentukan di query
	CounterName string  `json:"counter_name"`
	TotalOrder  float64 `json:"total_order"` // Tetap float64, akan diisi 0.0 jika NULL dari DB
	CreatedBy   string  `json:"created_by"`
	Tanggal     string  `json:"tanggal"` // Menggunakan string karena format sudah ditentukan di query
	IsApprove   *bool   `json:"is_approve"` // Gunakan pointer untuk handle NULL (bisa null di database)
}

// GetOrderDataHandler executes the provided SQL query and returns results as JSON.
func GetOrderDataHandler(c *fiber.Ctx) error {
	// Dapatkan koneksi database dari package db
	database := db.GetDB()

	// Pastikan koneksi database sudah diinisialisasi
	if database == nil {
		log.Println("Database connection is not initialized in db package.")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database connection is not initialized.",
		})
	}

	// Query SQL yang Anda berikan
	query := `
		SELECT
			DISTINCT t1.doc_id,
			t1.whs_code,
			(SELECT TOP(1) FORMAT(a1.created, 'dd-MM-yyyy HH:mm') AS created FROM  [pk-program].db_pgr.dbo.tb_order a1 WHERE a1.doc_id = t1.doc_id) AS created,
			(SELECT t2.counter_name FROM  [pk-program].db_pgr.dbo.master_warehouse t2 WHERE t1.whs_code = t2.whs_code) AS counter_name,
			(SELECT TOP(1) SUM(a1.qty_order) AS tot FROM  [pk-program].db_pgr.dbo.tb_order a1 WHERE a1.doc_id = t1.doc_id) AS total_order,
			t1.created_by,
			(SELECT TOP(1) FORMAT(a1.created, 'dd-MM-yyyy HH:mm') AS tanggal FROM  [pk-program].db_pgr.dbo.tb_order a1 WHERE a1.doc_id = t1.doc_id) AS tanggal,
			t1.is_approve
		FROM
			 [pk-program].db_pgr.dbo.tb_order t1
		WHERE
			is_approve IS NOT NULL
		ORDER BY
			created DESC
	`

	rows, err := database.Query(query) // Gunakan 'database' yang didapat dari db.GetDB()
	if err != nil {
		log.Printf("Error executing query: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to execute query",
			"details": err.Error(),
		})
	}
	defer rows.Close() // Pastikan rows ditutup setelah selesai

	var orders []DocOrder
	for rows.Next() {
		var order DocOrder
		var isApproveString sql.NullString  // Untuk menangani nilai string yang mungkin NULL dari is_approve
		var totalOrderFloat sql.NullFloat64 // Untuk menangani nilai float yang mungkin NULL dari total_order

		// Scan hasil query ke struct DocOrder
		err := rows.Scan(
			&order.DocID,
			&order.WhsCode,
			&order.Created,
			&order.CounterName,
			&totalOrderFloat,   // Scan ke sql.NullFloat64
			&order.CreatedBy,
			&order.Tanggal,
			&isApproveString, // Scan ke sql.NullString
		)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to scan row data",
				"details": err.Error(),
			})
		}

		// Konversi sql.NullFloat64 ke float64
		if totalOrderFloat.Valid {
			order.TotalOrder = totalOrderFloat.Float64
		} else {
			order.TotalOrder = 0.0 // Jika NULL di database, set ke 0.0
		}

		// Konversi sql.NullString ke *bool
		if isApproveString.Valid {
			if isApproveString.String == "y" {
				trueVal := true // Helper variable for pointer
				order.IsApprove = &trueVal
			} else if isApproveString.String == "n" {
				falseVal := false // Helper variable for pointer
				order.IsApprove = &falseVal
			} else {
				// Handle unexpected string values if necessary, e.g., log a warning
				log.Printf("Unexpected value for is_approve: %s. Setting to nil.", isApproveString.String)
				order.IsApprove = nil
			}
		} else {
			order.IsApprove = nil // Jika NULL di database, set ke nil di struct
		}

		orders = append(orders, order)
	}

	// Periksa error setelah iterasi rows
	if err = rows.Err(); err != nil {
		log.Printf("Error during rows iteration: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Error during data retrieval",
			"details": err.Error(),
		})
	}

	// Mengembalikan hasil sebagai JSON
	return c.JSON(orders)
}
