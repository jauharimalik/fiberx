package handlers

import (
	"database/sql" // Import sql package for sql.NullTime
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	_ "github.com/denisenkom/go-mssqldb" // Adjust with your database driver
	"my-fiber-app/db"                   // Assuming your db connection logic is here
)

// BirthdayEmployeeData extends EmployeeData to include DateOfBirth for birthday specific logic.
type BirthdayEmployeeData struct {
	EmployeeData
	DateOfBirth sql.NullTime `json:"dateOfBirth"`
}

// SendBirthdayWishes is a Fiber handler to fetch employees with upcoming birthdays
// and send personalized WhatsApp birthday wishes.
func SendBirthdayWishes(c *fiber.Ctx) error {
	// Get 'no' from query parameters for the recipient, or use a default.
	// In a real scenario, you might fetch this from a configuration or specific recipient list.
	no := c.Query("no")
	if no == "" {
		no = "085781550337" // Default number if not provided, for testing/example
	}

	// Assume db.DB is an already initialized database connection.
	dbConn := db.DB
	if dbConn == nil {
		log.Println("Database connection is nil. Ensure db.Connect() was called.")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database connection is nil. Ensure db.Connect() was called.",
		})
	}

	// SQL Query to fetch employees with birthdays in the current month or next 7 days, including today.
	// This ensures we capture birthdays that are today or coming up very soon.
	sqlQuery := `
        SELECT
            t0.User_ID AS Number,
            t0.Full_Name AS FullName,
            CONVERT(DATETIME, t0.Join_Date) AS JoinDate,
            CONVERT(DATETIME, t0.End_Contract_Probation_Expire_Date) AS EndEffectiveDate,
            t1.level_id,
            t1.whatsapp,
            (SELECT Department FROM [appsrv].pkap.dbo.master_department t0x WHERE t0x.id = t1.department_id) AS department,
            ISNULL(
                (SELECT TOP 1 fullname
                FROM [appsrv].pkap.dbo.master_users tx
                WHERE tx.department_id = t1.department_id
                AND level_id <= 3
                AND level_id >= 2
                ORDER BY level_id ASC),
                (SELECT TOP 1 fullname
                FROM [appsrv].pkap.dbo.master_users tx
                WHERE tx.department_id = 5
                AND level_id <= 3
                AND level_id >= 2
                ORDER BY level_id ASC)
            ) AS head,
            CONVERT(DATETIME, t0.Date_Of_Birth) AS ultah, -- DateOfBirth
            (SELECT job_position FROM [appsrv].pkap.dbo.master_position t0x WHERE t0x.id = t1.position_id) AS posisi
        FROM [PK_ANP_DEV_QUERY].[dbo].[master_karyawan_xls] t0
        left JOIN [appsrv].pkap.dbo.master_users t1 ON t0.User_ID = t1.username
        WHERE (
            (
                MONTH(GETDATE()) = MONTH(CONVERT(DATE, t0.Date_Of_Birth)) -- Bulan saat ini
                AND DAY(CONVERT(DATE, t0.Date_Of_Birth)) >= DAY((DAY(GETDATE()) - 2)) -- Mulai dari hari ini
                AND DAY(CONVERT(DATE, t0.Date_Of_Birth)) <= (DAY(GETDATE()) + 7) -- Hingga 7 hari ke depan
            )
            OR
            (
                -- Jika bulan depan masuk dalam rentang 7 hari dari hari ini (misal hari ini tgl 28, dan ada ultah tgl 3 bulan depan)
                -- Pastikan GETDATE() + 7 hari berada di bulan yang sama dengan tanggal lahir karyawan
                MONTH(DATEADD(DAY, 7, GETDATE())) = MONTH(CONVERT(DATE, t0.Date_Of_Birth))
                AND DAY(CONVERT(DATE, t0.Date_Of_Birth)) <= DAY(DATEADD(DAY, 7, GETDATE()))
                AND MONTH(GETDATE()) != MONTH(DATEADD(DAY, 7, GETDATE())) -- Only for跨月 birthdays
            )
        )
        ORDER BY MONTH(CONVERT(DATE, t0.Date_Of_Birth)), DAY(CONVERT(DATE, t0.Date_Of_Birth)) ASC;
    `

	rows, err := dbConn.Query(sqlQuery)
	if err != nil {
		log.Printf("Failed to run database query: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to retrieve birthday data: %v", err),
		})
	}
	defer rows.Close()

	var birthdayEmployees []BirthdayEmployeeData
	for rows.Next() {
		var bed BirthdayEmployeeData
		var joinDateTemp sql.NullTime
		var effectiveDateTemp sql.NullTime
		var ultahTemp sql.NullTime

		err := rows.Scan(
			&bed.Number,
			&bed.FullName,
			&joinDateTemp,
			&effectiveDateTemp,
			&bed.LevelID,
			&bed.Whatsapp,
			&bed.Department,
			&bed.Head,
			&ultahTemp, 
			&bed.Posisi,
		)
		if err != nil {
			log.Printf("Error scanning row for birthday data: %v", err)
			continue // Continue to the next row if scanning fails
		}

		bed.JoinDate = joinDateTemp
		bed.EndEffectiveDate = effectiveDateTemp
		bed.DateOfBirth = ultahTemp // Assign scanned birthday

		birthdayEmployees = append(birthdayEmployees, bed)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error after row iteration for birthday data: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Error processing birthday data: %v", err),
		})
	}

	// Generate WhatsApp message for birthdays
	pesanUltah := ""
	if len(birthdayEmployees) >= 0 {
		pesanUltah = "\n=============================\n🎉 Ucapan Ulang Tahun Karyawan 🎉\n=============================\n"
		for _, emp := range birthdayEmployees {
			birthdayStr := "N/A"
			age := 0
			if emp.DateOfBirth.Valid {
				birthdayStr = emp.DateOfBirth.Time.Format("02 January") // e.g., "02 January"
				// Calculate age based on current year and birth year
				age = time.Now().Year() - emp.DateOfBirth.Time.Year()
				// Adjust age if birthday hasn't occurred yet this year
				if time.Now().Month() < emp.DateOfBirth.Time.Month() ||
					(time.Now().Month() == emp.DateOfBirth.Time.Month() && time.Now().Day() < emp.DateOfBirth.Time.Day()) {
					age--
				}
			}

			pesanUltah += fmt.Sprintf(
				"\nSelamat Ulang Tahun ke-%d untuk:\nNama: %s\nWhatsapp: %s\nDepartemen: %s\nJabatan: %s\nTanggal Lahir: %s\nSemoga panjang umur, sehat selalu, dan sukses dalam karir!\n=============================\n",
				age,
				emp.FullName,
				emp.Whatsapp,
				emp.Department,
				emp.Posisi,
				birthdayStr,
			)
		}
	} else {
		pesanUltah = "Tidak ada karyawan yang berulang tahun dalam periode ini."
	}

	// Send the birthday message to WhatsApp API
	_, err = sendToWhatsAppAPI(no, pesanUltah, "") // No file attached for birthday wishes
	if err != nil {
		log.Printf("Error sending WhatsApp birthday message: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to send WhatsApp birthday notification: %v", err.Error()),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"no":      no,
		"pesan":   pesanUltah,
		"data":    birthdayEmployees, // Return the data retrieved from the database
	})
}
