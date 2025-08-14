package handlers

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"my-fiber-app/db"
)

func DeleteskpHandler(c *fiber.Ctx) error {
	if db.DB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(Response{
			Success: false,
			Message: "Koneksi database tidak tersedia.",
		})
	}
	query := `DELETE FROM tb_proposal_skp WHERE img IS NULL OR img = ''`
	result, err := db.DB.Exec(query)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(Response{
			Success: false,
			Message: fmt.Sprintf("Gagal menghapus data: %v", err),
		})
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(Response{
			Success: false,
			Message: "Gagal mengonfirmasi penghapusan.",
		})
	}
	if rowsAffected == 0 {
		return c.Status(fiber.StatusOK).JSON(Response{
			Success: true,
			Message: "Tidak ada baris yang terpengaruh, tidak ada data yang perlu dihapus.",
		})
	}
	return c.Status(fiber.StatusOK).JSON(Response{
		Success: true,
		Message: fmt.Sprintf("%d baris berhasil dihapus.", rowsAffected),
	})
}

func RunDeleteSkpTask() {
	if db.DB == nil {
		return
	}
	query := `DELETE FROM tb_proposal_skp WHERE img IS NULL OR img = '' or img like '%?%'`
	result, err := db.DB.Exec(query)
	if err != nil {
		return
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return
	}
	if rowsAffected > 0 {
		return
	}
}
