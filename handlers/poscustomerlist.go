package handlers

import (
	"database/sql"
	"log"

	"github.com/gofiber/fiber/v2"
	_ "github.com/denisenkom/go-mssqldb" // Ensure you have this driver imported
	"my-fiber-app/db" // Assuming your database connection utility is here
)

type Customerpos struct {
	CustomerID string        `json:"customer_id"`
	Name       string        `json:"name"`
	Email      string        `json:"email"`
	Phone      string        `json:"phone"`
	Birthday   sql.NullString `json:"birthday"` // Use sql.NullString for potentially null dates
	Created    sql.NullString `json:"created"`  // Use sql.NullString for potentially null dates
	Toko       sql.NullString `json:"toko"`     // Use sql.NullString for potentially null strings
	PointB     int           `json:"pointb"`
	Total      sql.NullString `json:"total"`    // Use sql.NullString for formatted currency
}


func CustomerLoad(c *fiber.Ctx) error {
	database := db.GetDB()
	if database == nil {
		log.Println("Database connection is not initialized.")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database connection is not initialized.",
		})
	}


		// SELECT
		// 	customer_id,
		// 	name,
		// 	email,
		// 	phone,
		// 	CONVERT(VARCHAR(10), birthday, 105) as birthday,
		// 	CONVERT(VARCHAR(10), created, 105) as created,
		// 	(SELECT TOP(1) nama_toko FROM [pk-program].db_pgr.dbo.tb_sales_summary WHERE nomor_member = customer.phone ORDER BY tanggal_transaksi ASC) as toko,
		// 	ISNULL((SELECT TOP(1) point_akhir FROM [pk-program].db_pgr.dbo.tb_trans_point WHERE nomor_member = customer.phone ORDER BY idt DESC), 0) AS pointb,
		// 	ISNULL((SELECT FORMAT(SUM(grand_total), 'C', 'id-ID') FROM [pk-program].db_pgr.dbo.tb_sales_summary WHERE nomor_member = customer.phone), 0) AS total
		// FROM [pk-program].db_pgr.dbo.customer

	sql0 := `
		SELECT distinct
			c.customer_id,
			c.name,
			c.email,
			c.phone,
			CONVERT(VARCHAR(10), c.birthday, 105) AS birthday,
			CONVERT(VARCHAR(10), c.created, 105) AS created,
			COALESCE(ss_first.nama_toko, '') AS toko, -- Use COALESCE for NULL handling
			COALESCE(tp_latest.point_akhir, 0) AS pointb,
			COALESCE(FORMAT(ss_total.total_grand_total, 'C', 'id-ID'), '0') AS total 
		FROM
			[pk-program].db_pgr.dbo.customer c
		LEFT JOIN (
			SELECT
				nomor_member,
				MIN(tanggal_transaksi) AS first_transaction_date
			FROM
				[pk-program].db_pgr.dbo.tb_sales_summary
			GROUP BY
				nomor_member
		) AS ss_min_date ON c.phone = ss_min_date.nomor_member
		LEFT JOIN [pk-program].db_pgr.dbo.tb_sales_summary ss_first
			ON c.phone = ss_first.nomor_member AND ss_first.tanggal_transaksi = ss_min_date.first_transaction_date
		LEFT JOIN (
			SELECT
				nomor_member,
				SUM(grand_total) AS total_grand_total
			FROM
				[pk-program].db_pgr.dbo.tb_sales_summary
			GROUP BY
				nomor_member
		) AS ss_total ON c.phone = ss_total.nomor_member
		LEFT JOIN (
			SELECT
				nomor_member,
				point_akhir,
				ROW_NUMBER() OVER (PARTITION BY nomor_member ORDER BY idt DESC) AS rn
			FROM
				[pk-program].db_pgr.dbo.tb_trans_point
		) AS tp_latest ON c.phone = tp_latest.nomor_member AND tp_latest.rn = 1
	`

	rows, err := database.Query(sql0)
	if err != nil {
		log.Printf("Error executing query: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to execute query",
			"details": err.Error(),
		})
	}
	defer rows.Close()

	var customerList []Customerpos
	for rows.Next() {
		var customerpos Customerpos
		// Use sql.NullString for columns that might return NULL from the DB
		err := rows.Scan(
			&customerpos.CustomerID,
			&customerpos.Name,
			&customerpos.Email,
			&customerpos.Phone,
			&customerpos.Birthday, // Scans into sql.NullString
			&customerpos.Created,  // Scans into sql.NullString
			&customerpos.Toko,     // Scans into sql.NullString
			&customerpos.PointB,
			&customerpos.Total,    // Scans into sql.NullString
		)
		if err != nil {
			log.Printf("Error scanning customer row: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to scan customer row data",
				"details": err.Error(),
			})
		}
		customerList = append(customerList, customerpos)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error during customer rows iteration: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Error during data retrieval",
			"details": err.Error(),
		})
	}

	return c.JSON(customerList)
}