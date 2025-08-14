package handlers

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// sendToWhatsAppAPI adalah fungsi helper untuk mengirim pesan ke API WhatsApp
func sendToWhatsAppAPI(number, message, filePath string) ([]byte, error) {
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
			return nil, fmt.Errorf("gagal membuka file: %w", err)
		}
		defer file.Close()

		part, err := writer.CreateFormFile("file_dikirim", filepath.Base(filePath))
		if err != nil {
			return nil, fmt.Errorf("gagal membuat form file: %w", err)
		}
		_, err = io.Copy(part, file)
		if err != nil {
			return nil, fmt.Errorf("gagal menyalin file ke form: %w", err)
		}
	}

	writer.Close()

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal mengirim request HTTP: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("respon API tidak sukses: status %d, body: %s", resp.StatusCode, responseBody)
	}

	return responseBody, nil
}

// SendWaJs2 adalah handler Fiber untuk mengirim pesan WhatsApp dengan atau tanpa lampiran.
func SendWaJs2(c *fiber.Ctx) error {
	// Ambil nomor dari query parameter atau body, atau gunakan nilai default jika tidak ada
	no := c.Query("no")
	if no == "" {
		// Jika tidak ada di query, coba ambil dari body (jika Anda mengirim JSON atau form)
		// atau set nilai default jika tidak ada
		// no = c.FormValue("no") // Untuk form-urlencoded
		// or
		// type WaRequest struct {
		// 	No    string `json:"no"`
		// 	Pesan string `json:"pesan"`
		// 	Gbfa  string `json:"gbfa"`
		// }
		// var req WaRequest
		// if err := c.BodyParser(&req); err == nil {
		// 	no = req.No
		// }
		// no = "085781550337" // Contoh nilai default
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Parameter 'no' (nomor tujuan) tidak boleh kosong.",
		})
	}

	// Format nomor telepon
	nomorTujuan := no
	if strings.HasPrefix(no, "62") {
		nomorTujuan = "0" + strings.TrimPrefix(no, "62")
	}

	// Ambil pesan dari query parameter atau body
	pesan := c.Query("pesan")
	if pesan == "" {
		// pesan = c.FormValue("pesan")
		// if req.Pesan != "" {
		// 	pesan = req.Pesan
		// }
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Parameter 'pesan' tidak boleh kosong.",
		})
	}
	pesan = strings.ReplaceAll(pesan, "<br>", "\n")

	// Ambil jalur file dari query parameter atau body
	gbfa := c.Query("gbfa")
	// If you want to handle file uploads directly via Fiber, you would use c.FormFile("file_dikirim")
	// and save it temporarily or process its content.
	// For now, assuming 'gbfa' is a path on the server.

	var targetDir string
	if gbfa != "" {
		// FCPATH di PHP perlu diterjemahkan ke jalur absolut di Go.
		// Ini adalah contoh sederhana, Anda mungkin perlu menentukan FCPATH sesuai dengan struktur proyek Anda.
		// Misalnya, jika 'gbfa' adalah relatif terhadap root proyek Anda:
		// currentDir, _ := os.Getwd()
		// targetDir = filepath.Join(currentDir, gbfa)
		targetDir = gbfa // Asumsi gbfa sudah merupakan path absolut atau relatif yang dapat diakses

		// Normalisasi path
		targetDir = filepath.Clean(targetDir)

		// Cek apakah file ada
		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			targetDir = "" // Set to empty if file doesn't exist, similar to PHP's behavior
		}
	}

	var responseBody []byte
	var err error

	if targetDir != "" {
		// Kirim dengan file
		responseBody, err = sendToWhatsAppAPI(nomorTujuan, pesan, targetDir)
	} else {
		// Kirim tanpa file
		responseBody, err = sendToWhatsAppAPI(nomorTujuan, pesan, "")
	}

	if err != nil {
		fmt.Printf("Error sending WhatsApp message: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Gagal mengirim pesan WhatsApp: %v", err.Error()),
		})
	}

	// Mengembalikan respons dari API WhatsApp
	return c.JSON(fiber.Map{
		"success":  true,
		"response": string(responseBody),
	})
}