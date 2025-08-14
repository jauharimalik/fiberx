package handlers


import (
	"github.com/gofiber/fiber/v2"
)
// Demo1Handler mengambil data dari tabel oitm.
func Demo1Handler(c *fiber.Ctx) error {
	query := "SELECT ItemCode, ItemName, FrgnName FROM pandurasa_live.dbo.oitm"
	// Tidak ada cache di sini
	return GenericQueryHandler(c, query)
}