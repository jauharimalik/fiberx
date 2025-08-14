package handlers

import (
	"github.com/gofiber/fiber/v2"
	"my-fiber-app/db"
)

func Demo1(c *fiber.Ctx) error {
	rows, err := db.DB.Query("SELECT * FROM OITM")
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()

	var data []map[string]interface{}

	cols, _ := rows.Columns()
	for rows.Next() {
		colsData := make([]interface{}, len(cols))
		colsPtr := make([]interface{}, len(cols))
		for i := range colsData {
			colsPtr[i] = &colsData[i]
		}

		rows.Scan(colsPtr...)

		rowMap := make(map[string]interface{})
		for i, col := range cols {
			val := colsData[i]
			rowMap[col] = val
		}
		data = append(data, rowMap)
	}

	return c.JSON(data)
}

func Demo2(c *fiber.Ctx) error {
	query := `-- tempelkan query panjang kamu di sini`
	rows, err := db.DB.Query(query)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()

	var data []map[string]interface{}

	cols, _ := rows.Columns()
	for rows.Next() {
		colsData := make([]interface{}, len(cols))
		colsPtr := make([]interface{}, len(cols))
		for i := range colsData {
			colsPtr[i] = &colsData[i]
		}

		rows.Scan(colsPtr...)

		rowMap := make(map[string]interface{})
		for i, col := range cols {
			val := colsData[i]
			rowMap[col] = val
		}
		data = append(data, rowMap)
	}

	return c.JSON(data)
}
