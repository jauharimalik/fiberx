package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Loadtabelproposal mengambil itemcode dari URL dan menjalankan query untuk mengambil detail item dari database.
func Loadtabelproposal(c *fiber.Ctx) error {
	// --- Query Parameters ---
	cmd := c.Query("cmd")
	durasi := c.Query("durasi") // Capture the 'durasi' parameter

	// Log the captured parameters for debugging
	fmt.Printf("LOG: cmd parameter: %s, durasi parameter: %s\n", cmd, durasi)

	// Determine the action based on the 'cmd' parameter
	if strings.ToLower(cmd) == "create" {
		// If cmd is "create", you would typically call a different function
		// or have specific write logic here.
		// As your current function only reads, we'll return an error or a specific response.
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "This endpoint is for reading data. 'create' operation is not handled here.",
		})
	}

	// Default behavior or if cmd is "read"
	// This block will execute for cmd="read" or when cmd is not provided.
	if strings.ToLower(cmd) == "read" || cmd == "" {
		userCode := c.Query("user_code")
		number := c.Query("number")
		endDate := c.Query("end_date")
		startDate := c.Query("start_date")
		fsYear := c.Query("fs_year") // fsYear captured here
		
		// Perbaikan: Tangkap 'group' sebagai multiple values
		groupValues := getQueryMulti(c, "group")

		statusValues := getQueryMulti(c, "status")
		activityValues := getQueryMulti(c, "activity")
		brandValues := getQueryMulti(c, "brand")

		// --- SQL Query Construction ---
		var conditions []string
		var queryParams []interface{}
		paramCounter := 1 // Start parameter indexing from 1 for SQL Server's @P1, @P2...

		query := `SELECT
			DISTINCT
			t1.id,
			t1.Number,
			t1.noref as Reff,
			tb.brandname as Brand,
			CONVERT(VARCHAR(10), t1.CreatedDate, 111)as [tgl_created],
			CONVERT(VARCHAR(10), t1.StartDatePeriode, 111)as [tgl_start],
			CONVERT(VARCHAR(10), t1.enddateperiode, 111)as [tgl_end],
			MONTH(t1.CreatedDate) AS [Month_Created],
			MONTH(t1.StartDatePeriode) AS [Month_Start],
			MONTH(t1.enddateperiode) AS [Month_End],
			YEAR(t1.CreatedDate) AS [Year_Created],
			YEAR(t1.StartDatePeriode) AS [Year_Start],
			YEAR(t1.enddateperiode) AS [Year_End],
			CONVERT(VARCHAR(10), t1.CreatedDate, 23) as CreatedDate,
			CONVERT(VARCHAR(10), t1.StartDatePeriode, 23) as [Start],
			CONVERT(VARCHAR(10), t1.enddateperiode, 23) as [End],
			mp.promo_name as Activity,
			(
				select top 1 t4.GroupName from tb_proposal_customer t3
				left join m_group t4 on t3.GroupCustomer = t4.GroupCode
				where t1.[Number] = t3.ProposalNumber
			)as [Group],

			(SELECT COUNT(id)
			FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_proposal_skp
			WHERE ProposalNumber = t1.Number and img is not null) AS Jml_SKP,
			(
				select distinct top 1
					ISNULL(ISNULL(
						NULLIF(tbc.kam, 'NULL'),
						ISNULL(
							NULLIF(tbc.spv, 'NULL'),
							ISNULL(NULLIF(tbc.smd, 'NULL'), NULLIF(tbc.rsm, 'NULL'))
						)
					),t2x.fullname) AS kam
				FROM  [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating_proposal t1x
				INNER JOIN [APPSRV].[PK_ANP_DEV_QUERY].dbo.master_user t2x ON t2x.username = t1x.CreatedBy
				LEFT JOIN  [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_proposal_customer tc ON tc.ProposalNumber = t1x.ProposalNumber
				LEFT JOIN [pk-query].db_santosh.dbo.b_cust tbc ON tbc.CardCode = tc.CustomerCode
				WHERE t1x.ProposalNumber = t1.number
			) AS Kam,
			isnull(t2.costing_lama,t2.TotalCosting) as [Costing],
			isnull(t2.realisasi,0) as [Realisasi],
			isnull((
				select top 1 concat('#'+tskp.noskp,' '+ CONVERT(VARCHAR, tskp.CreatedAt, 23)) from tb_proposal_skp tskp where tskp.proposalnumber = t1.number  order by tskp.id desc
			),'') as Skp,
			isnull((
				select count(*) from tb_proposal_lampiran tlamp where tlamp .proposalnumber = t1.number
			),0) as Lampiran,
			UPPER(t1.CreatedBy) as Pic,
			UPPER(t2.status_proposal) as [Status],
			UPPER(t1.ClaimTo) as ClaimTo,
			UPPER(t2.klaimable) as Claimable,
			isnull(t2.jnbalikkan, t5.number) as NoCN,
			t2.Budget_type AS Sumber,
			t2.budget_type AS BudgetType,
			isnull(
				(
					select top 1 CONCAT(tpa.username, ' - ', CONVERT(VARCHAR(10), tpa.created_at, 23))
					from tb_proposal_approved tpa
					where tpa.proposalnumber =  t1.number order by tpa.id desc)
			,'') as Management,
					CASE
			WHEN t2.budget_type != 'on_top' THEN t2.budget_total_tahunan
			ELSE (select sum(budget_on_top) from [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_budget_on_top tx1 where tx1.budget_code = t2.BudgetCode)
		END AS Total_Budget,
		CASE
			WHEN t2.budget_type != 'on_top' THEN (t2.budget_total_tahunan - (SELECT SUM(TotalCosting)
						FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating_proposal
						WHERE BrandCode = t1.BrandCode AND BudgetCode = t1.BudgetCode and status_proposal != 'canceled' and Budget_type != 'on_top'
						AND ProposalNumber <= t1.Number AND fs_year = t2.fs_year))
			ELSE ((select sum(budget_on_top) from [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_budget_on_top tx1 where tx1.budget_code = t2.BudgetCode) - (SELECT SUM(TotalCosting)
						FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating_proposal
						WHERE BrandCode = t1.BrandCode AND BudgetCode = t1.BudgetCode and status_proposal != 'canceled' and Budget_type = 'on_top'
						AND ProposalNumber <= t1.Number AND fs_year = t2.fs_year))
		END AS Balance,
		(
			SELECT
			SUM(tx0.LineTotal)
			FROM
			[APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_anp_dn_masuk tx0
			inner JOIN [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_proposal_dn tdn1 ON tx0.numatcard = tdn1.nodn
			WHERE
			U_IDU_NoProposal = t1.Number or tdn1.proposalnumber = t1.Number
		) AS DN_Dibuat,
		(SELECT SUM(tx0.jmlbyr)
		FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_anp_dn_dibayar tx0
		INNER JOIN [APPSRV].[PK_ANP_DEV_QUERY].dbo.m_brand_anp tx1
		ON tx0.kdbrand = tx1.code
		WHERE U_pk_noproposal = t1.Number AND tglbyr IS NOT NULL
		AND kdbrand IS NOT NULL AND tglbyr >= '2023-01-01'
		AND tx1.BrandCode = t1.BrandCode
		AND YEAR(tglbyr) = YEAR(t1.StartDatePeriode)
		GROUP BY YEAR(tglbyr), kdbrand) AS DN_Dibayar
			FROM
			tb_proposal t1
			inner join tb_operating_proposal t2 on t1.[Number] = t2.ProposalNumber
			inner join m_brand tb on tb.BrandCode = t1.BrandCode
			inner join m_promo mp on mp.id = t2.ActivityCode
			left join tb_anp_cn_potongan t5 on t1.Number = t5.u_idu_noproposal
			WHERE 1=1` // Start with 1=1 to make adding AND clauses easier

		// Condition for BrandCode based on UserCode
		if userCode != "" {
			conditions = append(conditions, fmt.Sprintf("t1.BrandCode IN (SELECT BrandCode FROM tb_pic_brand WHERE UserCode = @P%d)", paramCounter))
			queryParams = append(queryParams, userCode)
			paramCounter++
		}

		// --- Dynamic WHERE clauses ---
		if number != "" {
			conditions = append(conditions, fmt.Sprintf("t1.Number LIKE @P%d", paramCounter))
			queryParams = append(queryParams, "%"+number+"%")
			paramCounter++
		}

		if startDate != "" {
			conditions = append(conditions, fmt.Sprintf("CAST(t1.StartDatePeriode AS DATE) >= @P%d", paramCounter))
			queryParams = append(queryParams, startDate)
			paramCounter++
		} else {
			// Set default start date if not provided
			defaultStartDate := "2025-01-01" // Or use a dynamic date based on current year/month
			conditions = append(conditions, fmt.Sprintf("CAST(t1.StartDatePeriode AS DATE) >= @P%d", paramCounter))
			queryParams = append(queryParams, defaultStartDate)
			paramCounter++
		}

		if endDate != "" {
			conditions = append(conditions, fmt.Sprintf("CAST(t1.enddateperiode AS DATE) <= @P%d", paramCounter))
			queryParams = append(queryParams, endDate)
			paramCounter++
		}

		// Handling fsYear for StartDatePeriode
		// Prioritize provided fsYear, otherwise use current year
		targetYear := fsYear
		if targetYear == "" {
			targetYear = fmt.Sprintf("%d", time.Now().Year()) // Get current year as string
		}
		conditions = append(conditions, fmt.Sprintf("YEAR(t1.StartDatePeriode) = @P%d", paramCounter))
		queryParams = append(queryParams, targetYear)
		paramCounter++

		// Handling array input for Status
		if len(statusValues) > 0 {
			addInClause(&conditions, &queryParams, &paramCounter, "UPPER(t2.status_proposal)", statusValues, true) // Pass true for uppercase
		} else {
			conditions = append(conditions, fmt.Sprintf("t2.status_proposal != 'canceled'"))
		}

		// Handling array input for Activity
		if len(activityValues) > 0 {
			addInClause(&conditions, &queryParams, &paramCounter, "mp.id", activityValues, false) // Pass false for no uppercase
		}

		// Handling array input for Brand
		if len(brandValues) > 0 {
			addInClause(&conditions, &queryParams, &paramCounter, "tb.brandcode", brandValues, false) // Pass false for no uppercase
		}

		// Perbaikan: Handling array input for Group
		if len(groupValues) > 0 {
			// Using STRING_SPLIT for multiple values in a subquery for GroupCode
			conditions = append(conditions, fmt.Sprintf(`EXISTS (
				SELECT 1 FROM tb_proposal_customer t3
				INNER JOIN m_group t4 ON t3.GroupCustomer = t4.GroupCode
				WHERE t1.[Number] = t3.ProposalNumber
				AND t4.GroupCode IN (SELECT value FROM STRING_SPLIT(@P%d, ','))
			)`, paramCounter))
			// Join the group values into a single comma-separated string
			queryParams = append(queryParams, strings.Join(groupValues, ","))
			paramCounter++
		}
		
		// Combine all conditions
		for _, cond := range conditions {
			query += " AND " + cond
		}

		// --- ORDER BY clause ---
		query += `
			ORDER BY
			t1.id DESC
		`

		fmt.Println(query)
		fmt.Println("LOG: Parameter yang akan digunakan:", queryParams)
		return GenericQueryHandler(c, query, queryParams...)
	}

	// If 'cmd' is set to something other than 'read' or 'create'
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"error": "Invalid 'cmd' parameter. Accepted values are 'read', 'create', or empty (defaults to 'read').",
	})
}

// Helper function to get multiple query parameters
func getQueryMulti(c *fiber.Ctx, key string) []string {
	var values []string
	for _, b := range c.Context().QueryArgs().PeekMulti(key) {
		values = append(values, string(b))
	}
	return values
}

// Helper function to add IN clause for array inputs
// toUpper: true if the column value should be converted to UPPERCASE before comparison
func addInClause(conditions *[]string, queryParams *[]interface{}, paramCounter *int, column string, values []string, toUpper bool) {
	if len(values) == 0 {
		return // No values, nothing to add
	}

	placeholders := make([]string, len(values))
	for i := range values {
		placeholders[i] = fmt.Sprintf("@P%d", *paramCounter)
		*paramCounter++
	}

	*conditions = append(*conditions, fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ",")))

	for _, val := range values {
		processedVal := strings.TrimSpace(val)
		if toUpper {
			processedVal = strings.ToUpper(processedVal)
		}
		*queryParams = append(*queryParams, processedVal)
	}
}