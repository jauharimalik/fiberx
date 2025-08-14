package handlers

import (
	"bytes"
	"database/sql" // Import sql package for sql.NullTime
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	_ "github.com/denisenkom/go-mssqldb" // Adjust with your database driver
	"my-fiber-app/db" // Assuming your db connection logic is here
)

// EmployeeData represents the employee data structure from the SQL query.
type EmployeeData struct {
	Number           string       `json:"number"`
	FullName         string       `json:"fullName"`
	JoinDate         sql.NullTime `json:"joinDate"`
	EndEffectiveDate sql.NullTime `json:"endEffectiveDate"`
	LevelID          int          `json:"levelId"`
	Email            string       `json:"email"`
	Whatsapp         string       `json:"whatsapp"`
	Department       string       `json:"department"`
	Head             string       `json:"head"`
	Posisi           string       `json:"posisi"`
}

// UpdateKontrak is a Fiber handler to check employee contract updates and send WhatsApp notifications.
func UpdateKontrak(c *fiber.Ctx) error {
	// Get 'no' and 'pesan' from query parameters, or use default values.
	no := c.Query("no")
	if no == "" {
		no = "085781550337" // Default if not provided
	}
	pesan := c.Query("pesan")
	// If `pesan` is empty, it will be generated later based on employee data.

	// Assume db.DB is an already initialized database connection.
	dbConn := db.DB
	if dbConn == nil {
		log.Println("Database connection is nil. Ensure db.Connect() was called.")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database connection is nil. Ensure db.Connect() was called.",
		})
	}
	// Do NOT defer dbConn.Close() here if db.DB is a global, persistent connection.
	// It should be managed by your db connection pool/setup.

	sql0 := `
            SELECT
                t0.User_ID as Number,
                t0.Full_Name as FullName,
                CONVERT(DATETIME, t0.Join_Date) as JoinDate, -- EXPLICIT CONVERSION HERE
                CONVERT(DATETIME, t0.End_Contract_Probation_Expire_Date) as EndEffectiveDate, -- EXPLICIT CONVERSION HERE
                t1.level_id,
                t1.email,
                t1.whatsapp,
                (SELECT Department FROM [appsrv].pkap.dbo.master_department t0x WHERE t0x.id = t1.department_id) AS department,
                isnull((select top 1 fullname from [appsrv].pkap.dbo.master_users tx where tx.department_id = t1.department_id and level_id <= 3 and level_id >= 2 order by level_id asc),(select top 1 fullname from [appsrv].pkap.dbo.master_users tx where tx.department_id = 5 and level_id <= 3 and level_id >= 2 order by level_id asc)) as head,
                (SELECT job_position FROM [appsrv].pkap.dbo.master_position t0x WHERE t0x.id = t1.position_id) AS posisi
            FROM [PK_ANP_DEV_QUERY].[dbo].[master_karyawan_xls] t0
            INNER JOIN [appsrv].pkap.dbo.master_users t1 ON t0.User_ID = t1.username
            WHERE ((
                CONVERT(VARCHAR(10), DATEADD(MONTH, -3, t0.End_Contract_Probation_Expire_Date), 120) >= CONVERT(VARCHAR(10), DATEADD(DAY, -3, GETDATE()), 120) and
                    CONVERT(VARCHAR(10), DATEADD(MONTH, -3, t0.End_Contract_Probation_Expire_Date), 120) <= CONVERT(VARCHAR(10), GETDATE(), 120)
            ) or (
                CONVERT(VARCHAR(10), t0.End_Contract_Probation_Expire_Date, 120) BETWEEN CAST(GETDATE() AS DATE) AND DATEADD(DAY, 3, CAST(GETDATE() AS DATE))
            )
            )
            order by t1.department_id asc`

	// Using dbConn (global DB instance) to query
	rows, err := dbConn.Query(sql0)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to run database query: %v", err),
		})
	}
	defer rows.Close()

	var data []EmployeeData
	for rows.Next() {
		var ed EmployeeData
		var joinDateTemp sql.NullTime
		var effectiveDateTemp sql.NullTime 

		err := rows.Scan(
			&ed.Number,
			&ed.FullName,
			&joinDateTemp,      // Scan into the temporary sql.NullTime for JoinDate
			&effectiveDateTemp, // Scan into the temporary sql.NullTime for EndEffectiveDate
			&ed.LevelID,
			&ed.Email,
			&ed.Whatsapp,
			&ed.Department,
			&ed.Head,
			&ed.Posisi,
		)
		if err != nil {
			fmt.Printf("Error scanning row: %v\n", err)
			log.Printf("SQL Query that caused scan error: %s\n", sql0)
			continue // Continue to the next row if scanning fails
		}
        
        ed.JoinDate = joinDateTemp
        ed.EndEffectiveDate = effectiveDateTemp

		data = append(data, ed)
	}

	if err = rows.Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Error after row iteration: %v", err),
		})
	}

	if len(data) >= 1 {
		arrManager := []EmployeeData{}
		arrSpv := []EmployeeData{}
		arrStaff := []EmployeeData{}

		for _, dv := range data {
			posisiLower := strings.ToLower(dv.Posisi)
			if strings.Contains(posisiLower, "manager") {
				arrManager = append(arrManager, dv)
			} else if strings.Contains(posisiLower, "spv") {
				arrSpv = append(arrSpv, dv)
			} else {
				arrStaff = append(arrStaff, dv)
			}
		}

		if len(arrManager) >= 1 {
			if pesan == "" {
				pesan = "\n=============================\nPengangkatan Karyawan ke Permanent\n=============================\n"
			} else {
				pesan += "\n\n=============================\nPengangkatan Karyawan ke Permanent (Manager)\n=============================\n"
			}
			for _, dv := range arrManager {
				joinDateStr := "N/A"
				if dv.JoinDate.Valid {
					joinDateStr = dv.JoinDate.Time.Format("02/01/06") // dd/mm/yy
				}
				endDateStr := "N/A"
				if dv.EndEffectiveDate.Valid {
					endDateStr = dv.EndEffectiveDate.Time.Format("02/01/06") // dd/mm/yy
				}

				pesan += fmt.Sprintf(
					"\nNIK : %s\nNama : %s\nDepartment : %s\nJabatan : %s\nE-mail : %s\nWhatsapp : %s\nHead Department : %s\nTanggal Bergabung : %s\nTanggal Selesai : %s\n=============================",
					dv.Number,
					dv.FullName,
					dv.Department,
					dv.Posisi,
					dv.Email,
					dv.Whatsapp,
					dv.Head,
					joinDateStr,
					endDateStr,
				)
			}
		}

		if len(arrSpv) >= 1 {
			if pesan == "" {
				pesan = "\n\n=============================\nPembaharuan Kontrak Karyawan\n=============================\n"
			} else {
				pesan += "\n\n=============================\nPembaharuan Kontrak Karyawan (SPV)\n=============================\n"
			}
			for _, dv := range arrSpv {
				// Calculate selisihTahun only if JoinDate is valid
				selisihTahun := 0
				if dv.JoinDate.Valid {
					selisihTahun = time.Now().Year() - dv.JoinDate.Time.Year()
				}
				
				joinDateStr := "N/A"
				if dv.JoinDate.Valid {
					joinDateStr = dv.JoinDate.Time.Format("02/01/06") // dd/mm/yy
				}
				endDateStr := "N/A"
				if dv.EndEffectiveDate.Valid {
					endDateStr = dv.EndEffectiveDate.Time.Format("02/01/06") // dd/mm/yy
				}

				pesan += fmt.Sprintf(
					"\nNIK : %s\nNama : %s\nDepartment : %s\nJabatan : %s\nE-mail : %s\nWhatsapp : %s\nHead Department : %s\nTanggal Bergabung : %s\nTanggal Selesai : %s\nKontrak Ke : %d\n=============================",
					dv.Number,
					dv.FullName,
					dv.Department,
					dv.Posisi,
					dv.Email,
					dv.Whatsapp,
					dv.Head,
					joinDateStr,
					endDateStr,
					selisihTahun,
				)
			}
		}

		if len(arrStaff) >= 1 {
			if pesan == "" {
				pesan = "\n\n=============================\nPembaharuan Kontrak Karyawan\n=============================\n"
			} else {
				pesan += "\n\n=============================\nPembaharuan Kontrak Karyawan (Staff)\n=============================\n"
			}
			for _, dv := range arrStaff {
				// Calculate selisihTahun only if JoinDate is valid
				selisihTahun := 0
				if dv.JoinDate.Valid {
					selisihTahun = time.Now().Year() - dv.JoinDate.Time.Year()
				}

				joinDateStr := "N/A"
				if dv.JoinDate.Valid {
					joinDateStr = dv.JoinDate.Time.Format("02/01/06") // dd/mm/yy
				}
				endDateStr := "N/A"
				if dv.EndEffectiveDate.Valid {
					endDateStr = dv.EndEffectiveDate.Time.Format("02/01/06") // dd/mm/yy
				}

				pesan += fmt.Sprintf(
					"\nNIK : %s\nNama : %s\nDepartment : %s\nJabatan : %s\nE-mail : %s\nWhatsapp : %s\nHead Department : %s\nTanggal Bergabung : %s\nTanggal Selesai : %s\nKontrak Ke : %d\n=============================",
					dv.Number,
					dv.FullName,
					dv.Department,
					dv.Posisi,
					dv.Email,
					dv.Whatsapp,
					dv.Head,
					joinDateStr,
					endDateStr,
					selisihTahun,
				)
			}
		}

		// Call the sendToWhatsAppAPIx function
		_, err := sendToWhatsAppAPIx(no, pesan, "") // No file provided here
		if err != nil {
			fmt.Printf("Error sending WhatsApp message for updatekontrak: %v\n", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Failed to send WhatsApp notification: %v", err.Error()),
			})
		}
	} else {
		// If no matching employee data
		if pesan == "" {
			pesan = "No employees need contract renewal at this time."
		}
		// Send message without employee data
		_, err := sendToWhatsAppAPIx(no, pesan, "")
		if err != nil {
			fmt.Printf("Error sending WhatsApp message for no data: %v\n", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Failed to send WhatsApp notification (no data): %v", err.Error()),
			})
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"no":      no,
		"pesan":   pesan,
		"data":    data, // Return the data retrieved from the database as well
	})
}

// GetDB is a placeholder for obtaining a database connection.
// In a real application, this would likely be in a separate db package.
func GetDB() (*sql.DB, error) {
	if db.DB == nil {
		return nil, fmt.Errorf("database connection not initialized in 'db' package")
	}
	return db.DB, nil
}

// sendToWhatsAppAPIx sends a message to the WhatsApp API with or without a file.
func sendToWhatsAppAPIx(number, message, filePath string) ([]byte, error) {
	url := "http://103.169.73.3:4040/send-message"
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add 'number' field
	_ = writer.WriteField("number", number)
	// Add 'message' field
	_ = writer.WriteField("message", message)

	if filePath != "" {
		file, err := os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		part, err := writer.CreateFormFile("file_dikirim", filepath.Base(filePath))
		if err != nil {
			return nil, fmt.Errorf("failed to create form file: %w", err)
		}
		_, err = io.Copy(part, file)
		if err != nil {
			return nil, fmt.Errorf("failed to copy file to form: %w", err)
		}
	}

	writer.Close()

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unsuccessful API response: status %d, body: %s", resp.StatusCode, responseBody)
	}

	return responseBody, nil
}

// SendWaJs2Handler is a Fiber handler for sending WhatsApp messages with or without attachments.
// Renamed to avoid conflicts if another 'SendWaJs2' function exists in your project.
func SendWaJs2Handler(c *fiber.Ctx) error {
	// Get 'no' from query parameter or body, or return an error if empty
	no := c.Query("no")
	if no == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Parameter 'no' (destination number) cannot be empty.",
		})
	}

	// Format phone number
	nomorTujuan := no
	if strings.HasPrefix(no, "62") {
		nomorTujuan = "0" + strings.TrimPrefix(no, "62")
	}

	// Get 'pesan' from query parameter or body
	pesan := c.Query("pesan")
	if pesan == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Parameter 'pesan' cannot be empty.",
		})
	}
	pesan = strings.ReplaceAll(pesan, "<br>", "\n")

	// Get 'gbfa' (file path) from query parameter or body
	gbfa := c.Query("gbfa")

	var targetDir string
	if gbfa != "" {
		targetDir = gbfa // Assume 'gbfa' is already an absolute or accessible relative path

		// Normalize path
		targetDir = filepath.Clean(targetDir)

		// Check if the file exists
		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			targetDir = "" // Set to empty if file doesn't exist, similar to PHP's behavior
		}
	}

	var responseBody []byte
	var err error

	if targetDir != "" {
		// Send with file
		responseBody, err = sendToWhatsAppAPIx(nomorTujuan, pesan, targetDir)
	} else {
		// Send without file
		responseBody, err = sendToWhatsAppAPIx(nomorTujuan, pesan, "")
	}

	if err != nil {
		fmt.Printf("Error sending WhatsApp message: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to send WhatsApp message: %v", err.Error()),
		})
	}

	// Return the response from the WhatsApp API
	return c.JSON(fiber.Map{
		"success":  true,
		"response": string(responseBody),
	})
}