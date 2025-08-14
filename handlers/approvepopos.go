package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"my-fiber-app/db" // Import your custom db package
)

// Response struct defines the structure for JSON responses.
type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ApproveOrderHandler handles the approval of an order by its doc_id.
// It sets 'is_approve' to 'y', 'approved_by' to 'admin', and 'approved_at' to the current time.
// The doc_id is expected as a URL parameter (e.g., /approve-order/:doc_id).
func ApproveOrderHandler(c *fiber.Ctx) error {
	// Extract 'doc_id' from the URL parameters.
	noOrder := c.Query("doc_id") 
	if noOrder == "" {
		// Return a JSON error response if doc_id is missing.
		return c.Status(fiber.StatusBadRequest).JSON(Response{
			Success: false,
			Message: "Missing 'doc_id' parameter in URL.",
		})
	}

	// Get the current time for the 'approved_at' field.
	// We use time.Now() and format it to a string suitable for SQL Server datetime.
	// Alternatively, you could use GETDATE() directly in the SQL query if you prefer
	// the database server's time.
	approvedAt := time.Now().Format("2006-01-02 15:04:05") // YYYY-MM-DD HH:MM:SS format

	// Define the SQL UPDATE query.
	// We use named parameters (@paramName) which are standard for go-mssqldb.
	query := `
        UPDATE [pk-program].db_pgr.dbo.tb_order
        SET
            is_approve = @is_approve,
            approved_by = @approved_by,
            approved_at = @approved_at
        WHERE
            doc_id = @doc_id;
    `

	// Prepare the parameters for the query.
	// sql.Named is used to bind Go variables to the named parameters in the SQL query.
	params := []interface{}{
		sql.Named("is_approve", "y"),
		sql.Named("approved_by", "admin"), // Hardcoded as per your request
		sql.Named("approved_at", approvedAt),
		sql.Named("doc_id", noOrder),
	}

	// Ensure the database connection is initialized.
	if db.DB == nil {
		log.Println("Database connection not initialized.")
		// Return a JSON error response if database connection is not available.
		return c.Status(fiber.StatusInternalServerError).JSON(Response{
			Success: false,
			Message: "Database connection not available.",
		})
	}

	// Execute the UPDATE query.
	// db.DB.Exec is used for queries that do not return rows (like INSERT, UPDATE, DELETE).
	result, err := db.DB.Exec(query, params...)
	if err != nil {
		log.Printf("Error updating order %s: %v", noOrder, err)
		// Return a JSON error response if the database query fails.
		return c.Status(fiber.StatusInternalServerError).JSON(Response{
			Success: false,
			Message: fmt.Sprintf("Failed to approve order: %v", err),
		})
	}

	// Check the number of rows affected to confirm the update was successful.
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected for order %s: %v", noOrder, err)
		// Return a JSON error response if getting rows affected fails.
		return c.Status(fiber.StatusInternalServerError).JSON(Response{
			Success: false,
			Message: "Failed to confirm order approval.",
		})
	}

	if rowsAffected == 0 {
		// Return a JSON response if no rows were affected (order not found or already approved).
		return c.Status(fiber.StatusNotFound).JSON(Response{
			Success: false,
			Message: fmt.Sprintf("Order with doc_id '%s' not found or already approved.", noOrder),
		})
	}

	// Return a JSON success response if the order was approved successfully.
	return c.Status(fiber.StatusOK).JSON(Response{
		Success: true,
		Message: fmt.Sprintf("Order '%s' approved successfully.", noOrder),
	})
}
