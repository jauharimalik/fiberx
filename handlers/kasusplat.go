package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// KasusPlatHandler handles the specific Kasus Plat query with caching.
// It extracts query parameters from the URL and constructs the SQL query dynamically.
func KasusPlatHandler(c *fiber.Ctx) error {
	// Extract query parameters from the URL
	// Based on the PHP script, there are no direct query parameters used for WHERE clauses.
	// However, if you plan to add filters like 'document_number', 'subject', 'supplier_name', etc.,
	// you would extract them here, similar to 'noskp', 'fsYear', 'brand' in AnpApiXLHandler.
	// For now, we'll keep it without dynamic filters to match the provided PHP.

	// Construct a unique cache key for this handler.
	// If you add query parameters later, make sure to include them in the cache key.
	cacheKey := "KasusPlatHandler"

	// The base SQL query, copied directly from your PHP script.
	// NOTE: I've added aliases to all selected columns to make them explicit
	// and easier to map to the `desiredOrder` slice.
	baseSQL := `
SELECT 
    t1.document_number AS document_number,
    (
        SELECT TOP 1 tv.plate_number
        FROM [PKSRV-SAP].[PK_EXPRESS].[dbo].service_request tx 
        INNER JOIN [PKSRV-SAP].[PK_EXPRESS].[dbo].vehicle tv 
            ON tv.plate_number LIKE '%b ' + LEFT(dbo.GetOnlyDigits(CAST(tx.subject AS VARCHAR(MAX))), 4) + '%'
        WHERE 
            (tx.id = t1.service_request_id)
            OR (
                CAST(tx.subject AS VARCHAR(MAX)) LIKE '%' + CAST(t1.subject AS VARCHAR(MAX)) + '%'
                AND CONVERT(DATE, tx.created_date) = CONVERT(DATE, t1.created_date)
            )
    ) AS platnomor,
    t1.subject AS subject,
    t2.name AS produk,
    t0.price AS harga,
    ISNULL(t32.supplier_name, t3.supplier_name) AS supplier_name,
    (
        SELECT TOP 1 tx.description 
        FROM [PKSRV-SAP].[PK_EXPRESS].[dbo].service_request tx 
        WHERE 
            (tx.id = t1.service_request_id)
            OR (
                CAST(tx.subject AS VARCHAR(MAX)) LIKE '%' + CAST(t1.subject AS VARCHAR(MAX)) + '%'
                AND CONVERT(DATE, tx.created_date) = CONVERT(DATE, t1.created_date)
            )
    ) AS deskripsi,
    CONVERT(DATE, t1.created_date) AS tanggal,
    (
        SELECT TOP 1 tx.id 
        FROM [PKSRV-SAP].[PK_EXPRESS].[dbo].service_request tx 
        WHERE 
            (tx.id = t1.service_request_id)
            OR (
                CAST(tx.subject AS VARCHAR(MAX)) LIKE '%' + CAST(t1.subject AS VARCHAR(MAX)) + '%'
                AND CONVERT(DATE, tx.created_date) = CONVERT(DATE, t1.created_date)
            )
    ) AS service_id,
    t1.service_request_id AS service_request_id
FROM [PKSRV-SAP].[PK_EXPRESS].[dbo].purchase_request_d t0 
INNER JOIN [PKSRV-SAP].[PK_EXPRESS].[dbo].master_product t2 ON t2.sid = t0.product_sid
INNER JOIN [PKSRV-SAP].[PK_EXPRESS].[dbo].purchase_request t1 ON t0.purchase_request_id = t1.id
INNER JOIN [PKSRV-SAP].[PK_EXPRESS].[dbo].app_purchase_request tapr ON tapr.document_number = t1.document_number
INNER JOIN [PKSRV-SAP].[PK_EXPRESS].[dbo].supplier t3 ON t3.sid = t1.supplier_sid
INNER JOIN [PKSRV-SAP].[PK_EXPRESS].[dbo].supplier t32 ON t3.sid = tapr.supplier_sid
WHERE (
    t1.vehicle_code IS NOT NULL
    OR t1.subject LIKE '%serv%'
    OR t1.subject LIKE '%mobil%'
    OR t1.subject LIKE '% b %'
) 
AND t1.subject NOT LIKE '%forkli%'
AND t1.subject NOT LIKE '%forkli%'
AND t1.created_date >= '2024-01-01'`

	var conditions []string
	var params []interface{}
	// paramIndex := 1 // Not used currently as there are no dynamic parameters from URL

	// If you want to add query parameters for filtering, uncomment the following
	// and add your logic, similar to the AnpApiXLHandler.
	/*
		documentNumber := c.Query("document_number")
		if documentNumber != "" {
			conditions = append(conditions, fmt.Sprintf("t1.document_number LIKE @p%d", paramIndex))
			params = append(params, "%"+documentNumber+"%")
			paramIndex++
		}
	*/

	// If there are any conditions, append them to the base SQL query with "AND".
	if len(conditions) > 0 {
		baseSQL += " AND " + strings.Join(conditions, " AND ")
	}

	// Append the ORDER BY clause.
	baseSQL += " ORDER BY CONVERT(DATE, t1.created_date) DESC, t1.document_number DESC;"

	// Define the desired column order for the HTML table explicitly.
	// This MUST match the aliases used in your SQL SELECT statement.
	desiredOrder := []string{
		"document_number",
		"platnomor",
		"subject",
		"produk",
		"harga",
		"supplier_name",
		"deskripsi",
		"tanggal",
		"service_id",
		"service_request_id",
	}

	// Pass the constructed query, cache key, the desiredOrder slice, and parameters
	// to the GenericHtmlQueryHandler for caching and execution.
	return GenericHtmlQueryHandler(c, cacheKey, baseSQL, desiredOrder, params...)
}