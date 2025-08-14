package handlers

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func AnpBudgetSisaHandler(c *fiber.Ctx) error {
	// Extract query parameters from the URL
	fsYear := c.Query("fs_year")
	brand := c.Query("brand")

	// Construct a unique cache key based on the query parameters.
	var cacheKeyBuilder strings.Builder
	cacheKeyBuilder.WriteString("AnpBudgetSisaHandler") // Base prefix for this handler
	if fsYear != "" {
		cacheKeyBuilder.WriteString("_fsYear=")
		cacheKeyBuilder.WriteString(fsYear)
	}
	if brand != "" {
		cacheKeyBuilder.WriteString("_brand=")
		cacheKeyBuilder.WriteString(brand)
	}
	cacheKey := cacheKeyBuilder.String()

	// The base SQL query, adapted to use CTE and target specific tables.
	// We will build conditions dynamically and apply them to both parts of the UNION ALL.
	baseSQL := `
	WITH BrandBudget AS (
		SELECT DISTINCT
			t2x.BrandCode,
			(SELECT TOP 1 BudgetCode FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating_proposal t1x WHERE t1x.BudgetCode LIKE '%' + t2x.BrandCode + '%' AND t1x.BrandCode = t2x.BrandCode ORDER BY t1x.BudgetCode DESC) AS BudgetCode
		FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating_proposal t2x
		WHERE t2x.BudgetCode IS NOT NULL
			AND t2x.BudgetCode != ''
	)
	SELECT
		mpr.no AS Number,
		mpr.promo_name AS Activity,
		ISNULL((SELECT top 1 BrandName from [APPSRV].[PK_ANP_DEV_QUERY].dbo.m_brand where BrandCode = bb.BrandCode), '') AS Brand,
		ISNULL((
			SELECT max(toa.BudgetActivity)
			FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating_activity toa
			WHERE toa.ActivityCode = mpr.id
				AND toa.BrandCode = bb.BrandCode
				AND toa.BudgetCode = bb.BudgetCode
		), 0) AS Awal,
		ISNULL((
			SELECT SUM(topr.TotalCosting)
			FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating_proposal topr
			WHERE topr.ActivityCode = mpr.id
				AND topr.BrandCode = bb.BrandCode
				AND topr.BudgetCode = bb.BudgetCode
				AND topr.status_proposal != 'canceled'
		), 0) AS Terpakai,
		(
			ISNULL((
				SELECT max(toa.BudgetActivity)
				FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating_activity toa
				WHERE toa.ActivityCode = mpr.id
					AND toa.BrandCode = bb.BrandCode
					AND toa.BudgetCode = bb.BudgetCode
			), 0) -
			ISNULL((
				SELECT SUM(topr.TotalCosting)
				FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating_proposal topr
				WHERE topr.ActivityCode = mpr.id
					AND topr.BrandCode = bb.BrandCode
					AND topr.BudgetCode = bb.BudgetCode
					AND topr.status_proposal != 'canceled'
			), 0)
		) AS Sisa,
		bb.BrandCode,
		ISNULL((SELECT top 1 Pic from [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_pic_brand where BrandCode = bb.BrandCode and pic not like '%regis%' and pic not like '%deyan%'), '') AS Pic,
		bb.BudgetCode,
		(
			select top 1 topr.fs_year FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating topr
				WHERE topr.BrandCode = bb.BrandCode
					AND topr.BudgetCode = bb.BudgetCode
		) as Tahun
	FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.m_promo mpr
	CROSS JOIN BrandBudget bb
	WHERE 1=1 ` // Start with an always true condition to easily append AND clauses

	onTopSQL := `
	SELECT distinct
		'32' AS Number,
		'ON TOP' AS Activity,
		tb.BrandName AS Brand,
		ISNULL((
			SELECT SUM(tp2.budget_on_top) FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_budget_on_top tp2 WHERE tp2.budget_code = tp.budget_code
		), 0) AS Awal,
		ISNULL((
			SELECT SUM(tp2.TotalCosting) FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating_proposal tp2 WHERE tp2.BudgetCode = tp.budget_code AND tp2.Budget_type = 'on_top'
		), 0) AS Terpakai,
		ISNULL((
			(SELECT SUM(tp2.budget_on_top) FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_budget_on_top tp2 WHERE tp2.budget_code = tp.budget_code) -
			(SELECT SUM(tp2.TotalCosting) FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating_proposal tp2 WHERE tp2.BudgetCode = tp.budget_code AND tp2.Budget_type = 'on_top')
		), 0) AS Sisa,
		tp.brand_code AS BrandCode,
		ISNULL((
			SELECT TOP 1 tp2.Pic FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_pic_brand tp2 WHERE tp2.BrandCode = tp.brand_code AND (tp2.Pic NOT LIKE '%regis%' AND tp2.Pic NOT LIKE '%deya%' AND tp2.Pic NOT LIKE '%triana%' AND tp2.Pic NOT LIKE '%rara%')
		), '') AS Pic,
		tp.budget_code AS BudgetCode,
		ISNULL((
			SELECT TOP 1 tp2.fs_year FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating tp2 WHERE tp2.BudgetCode = tp.budget_code
		), 0) AS Tahun -- Corrected alias from 'Sisa' to 'Tahun'
	FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_budget_on_top tp
	INNER JOIN [APPSRV].[PK_ANP_DEV_QUERY].dbo.m_brand tb ON tb.BrandCode = tp.brand_code
	WHERE 1=1 ` // Start with an always true condition for onTopSQL

	var conditions []string
	var onTopConditions []string
	var params []interface{}
	paramIndex := 1 // Used for SQL Server's parameterized query syntax (@p1, @p2, etc.)

	// Add condition for 'brand' if present.
	if brand != "" {
		// Condition for the first part of the UNION ALL
		conditions = append(conditions, fmt.Sprintf("(bb.BrandCode = @p%d OR (SELECT TOP 1 BrandName FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.m_brand WHERE BrandCode = bb.BrandCode) LIKE @p%d)", paramIndex, paramIndex+1))
		params = append(params, brand, "%"+brand+"%")
		paramIndex += 2

		// Condition for the second part (ON TOP) of the UNION ALL
		onTopConditions = append(onTopConditions, fmt.Sprintf("(tp.brand_code = @p%d OR tb.BrandName LIKE @p%d)", paramIndex, paramIndex+1))
		params = append(params, brand, "%"+brand+"%")
		paramIndex += 2
	}

	// Add condition for 'fs_year' if present.
	if fsYear != "" {
		// Condition for the first part of the UNION ALL
		conditions = append(conditions, fmt.Sprintf("EXISTS (SELECT 1 FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating top_filter WHERE top_filter.BrandCode = bb.BrandCode AND top_filter.BudgetCode = bb.BudgetCode AND CAST(top_filter.fs_year AS VARCHAR) LIKE @p%d)", paramIndex))
		params = append(params, "%"+fsYear+"%")
		paramIndex++

		// Condition for the second part (ON TOP) of the UNION ALL
		onTopConditions = append(onTopConditions, fmt.Sprintf("EXISTS (SELECT 1 FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating top_filter WHERE top_filter.BudgetCode = tp.budget_code AND CAST(top_filter.fs_year AS VARCHAR) LIKE @p%d)", paramIndex))
		params = append(params, "%"+fsYear+"%")
		paramIndex++
	}

	// If there are any conditions for the first part, append them.
	if len(conditions) > 0 {
		baseSQL += " AND " + strings.Join(conditions, " AND ")
	}

	// If there are any conditions for the second part, append them.
	if len(onTopConditions) > 0 {
		onTopSQL += " AND " + strings.Join(onTopConditions, " AND ")
	}

	// Combine the two SQL queries with UNION ALL
	finalSQL := fmt.Sprintf("%s UNION ALL %s ORDER BY BrandCode, BudgetCode, Number;", baseSQL, onTopSQL)

	// Define the desired column order for the HTML table explicitly.
	desiredOrder := []string{
		"Number", "Activity", "Brand", "Awal", "Terpakai", "Sisa", "BrandCode", "Pic", "BudgetCode", "Tahun",
	}

	// Pass the 'desiredOrder' slice as the fourth argument.
	return GenericHtmlQueryHandler(c, cacheKey, finalSQL, desiredOrder, params...)
}