package handlers

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// AnpApiXLHandler handles the specific ANP API XL query with caching.
// It extracts query parameters from the URL and constructs the SQL query dynamically.
func AnpApiXLHandler(c *fiber.Ctx) error {
	// Extract query parameters from the URL
	noskp := c.Query("noskp")
	fsYear := c.Query("fs_year")
	brand := c.Query("brand")

	// Construct a unique cache key based on the query parameters.
	var cacheKeyBuilder strings.Builder
	cacheKeyBuilder.WriteString("AnpApiXLHandler") // Base prefix for this handler
	if noskp != "" {
		cacheKeyBuilder.WriteString("_noskp=")
		cacheKeyBuilder.WriteString(noskp)
	}
	if fsYear != "" {
		cacheKeyBuilder.WriteString("_fsYear=")
		cacheKeyBuilder.WriteString(fsYear)
	}
	if brand != "" {
		cacheKeyBuilder.WriteString("_brand=")
		cacheKeyBuilder.WriteString(brand)
	}
	cacheKey := cacheKeyBuilder.String()

	// The base SQL query, copied directly from your PHP script.
	// NOTE: I've added aliases to all selected columns to make them explicit
	// and easier to map to the `desiredOrder` slice.
	baseSQL := `
    SELECT DISTINCT
        t1.Number AS Number,
        (SELECT DISTINCT TOP(1) BrandName FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.m_brand WHERE BrandCode = t1.BrandCode) AS BrandName,
        t1.StartDatePeriode AS StartDatePeriode,
        t1.EndDatePeriode AS EndDatePeriode,
        (SELECT DISTINCT TOP(1) promo_name FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.m_promo WHERE id = t1.Activity) AS ActivityName,
        t1.Status AS Status,
        ISNULL(
            t1.CreatedBy,
            (SELECT TOP 1 CreatedBy FROM  [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating_proposal WHERE ProposalNumber = t1.Number)
        ) AS CreatedBy,
        t1.CreatedDate AS CreatedDate,
        tp1.costing_lama AS TotalCosting,
        tp1.TotalCosting AS CostingLama, -- Aliased for clarity, avoiding duplicate 'TotalCosting'
        tp1.jnbalikkan AS JnBalikkan,
        tp1.realisasi AS Credit,
        CASE
            WHEN tp1.budget_type != 'on_top' THEN tp1.budget_total_tahunan
            ELSE (select sum(budget_on_top) from [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_budget_on_top tx1 where tx1.budget_code = tp1.BudgetCode)
        END AS Total_Budget,
        CASE
            WHEN tp1.budget_type != 'on_top' THEN (tp1.budget_total_tahunan - (SELECT SUM(TotalCosting)
                                                FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating_proposal
                                                WHERE BrandCode = t1.BrandCode AND BudgetCode = t1.BudgetCode and status_proposal != 'canceled' and Budget_type != 'on_top'
                                                AND ProposalNumber <= t1.Number AND fs_year = tp1.fs_year))
            ELSE ((select sum(budget_on_top) from [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_budget_on_top tx1 where tx1.budget_code = tp1.BudgetCode) - (SELECT SUM(TotalCosting)
                                                FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating_proposal
                                                WHERE BrandCode = t1.BrandCode AND BudgetCode = t1.BudgetCode and status_proposal != 'canceled' and Budget_type = 'on_top'
                                                AND ProposalNumber <= t1.Number AND fs_year = tp1.fs_year))
        END AS Balance,
        (
            SELECT
            SUM(tx0.LineTotal)
            FROM
            [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_anp_dn_masuk tx0
            inner JOIN [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_proposal_dn tdn1 ON tx0.numatcard = tdn1.nodn
            WHERE
            U_IDU_NoProposal = t1.Number or tdn1.proposalnumber = t1.number
        ) AS DN_Dibuat,
        (SELECT SUM(tx0.jmlbyr)
        FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_anp_dn_dibayar tx0
        INNER JOIN [APPSRV].[PK_ANP_DEV_QUERY].dbo.m_brand_anp tx1
        ON tx0.kdbrand = tx1.code
        WHERE U_pk_noproposal = t1.Number AND tglbyr IS NOT NULL
        AND kdbrand IS NOT NULL AND tglbyr >= '2023-01-01'
        AND tx1.BrandCode = t1.BrandCode
        AND YEAR(tglbyr) = YEAR(t1.StartDatePeriode)
        GROUP BY YEAR(tglbyr), kdbrand) AS DN_Dibayar,

        null AS CN_Principal,
        STUFF((SELECT distinct '~~' + t6.GroupName
            FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_proposal_customer t5
            INNER JOIN [APPSRV].[PK_ANP_DEV_QUERY].dbo.m_group t6
            ON t6.GroupCode = t5.GroupCustomer
            WHERE t5.ProposalNumber = t1.Number
            FOR XML PATH(''), TYPE).value('.', 'NVARCHAR(MAX)'), 1, 2, '') AS GroupName,
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
        tp1.Budget_type AS Sumber,
        t1.ClaimTo AS KlaimKe,
        tp1.fs_year AS FsYear,
        tp1.budget_type AS BudgetType,
        STUFF((
            SELECT ' , ' + mc2.CardCode + ' - ' + mc2.CustomerName
            FROM  [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_proposal_customer tpc2
            INNER JOIN [APPSRV].[PK_ANP_DEV_QUERY].dbo.m_customer mc2 ON tpc2.CustomerCode = mc2.CardCode
            WHERE tpc2.ProposalNumber = t1.Number
            FOR XML PATH(''), TYPE
        ).value('.', 'VARCHAR(MAX)'), 1, 3, '') AS Toko,
    COALESCE(
        (SELECT TOP 1 tdn1.nodn
        FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_proposal_dn tdn1
        WHERE tdn1.proposalnumber = t1.number
        ORDER BY tdn1.nodn ASC),

        (SELECT TOP 1 numatcard
        FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_anp_dn_masuk
        WHERE U_IDU_NoProposal = t1.number
        ORDER BY numatcard ASC)
    ) AS NoDN,
        isnull((
            select max(t01.BudgetActivity) from  [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating_activity t01 where t01.ActivityCode = tp1.ActivityCode and t01.BrandCode = tp1.brandcode and t01.BudgetCode = tp1.budgetcode
        ),0) as BudgetActivity
    FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_proposal t1
    left JOIN [APPSRV].[PK_ANP_DEV_QUERY].dbo.tb_operating_proposal tp1
    ON t1.Number = tp1.ProposalNumber and t1.[Status] != 'canceled'
    left join [appsrv].pk_anp_dev_query.dbo.tb_proposal_approved t2
    on t1.number = t2.proposalnumber and t1.[Status] != 'canceled'
    where t1.number is not null`

	var conditions []string
	var params []interface{}
	paramIndex := 1 // Used for SQL Server's parameterized query syntax (@p1, @p2, etc.)

	// Add condition for 'noskp' if present in the URL query.
	if noskp != "" {
		conditions = append(conditions, `not exists(
			select * from [appsrv].pk_anp_dev_query.dbo.tb_proposal_skp t1x
			where (t1x.status_skp != 'canceled' or t1x.status_skp is null) and t1x.proposalnumber = t1.Number
		) and t1.startdateperiode >= '2024-01-01' and DATEADD(DAY, 5, t2.approvedDate) < getdate()`)
	}

	if brand != "" {
		conditions = append(conditions, fmt.Sprintf("(t1.BrandCode = @p%d OR (SELECT TOP 1 BrandName FROM [APPSRV].[PK_ANP_DEV_QUERY].dbo.m_brand WHERE BrandCode = t1.BrandCode) LIKE @p%d)", paramIndex, paramIndex+1))
		params = append(params, brand, "%"+brand+"%")
		paramIndex += 2
	}

	// Add condition for 'fs_year' if present.
	if fsYear != "" {
		conditions = append(conditions, fmt.Sprintf("(tp1.fs_year LIKE @p%d OR CONVERT(VARCHAR, t1.CreatedDate, 120) LIKE @p%d)", paramIndex, paramIndex+1))
		params = append(params, "%"+fsYear+"%", "%"+fsYear+"%")
		paramIndex += 2
	}

	// If there are any conditions, append them to the base SQL query with "AND".
	if len(conditions) > 0 {
		baseSQL += " AND " + strings.Join(conditions, " AND ")
	}

	// Append the ORDER BY clause.
	baseSQL += " ORDER BY t1.CreatedDate DESC;"

	// Define the desired column order for the HTML table explicitly.
	// This MUST match the aliases used in your SQL SELECT statement.
	desiredOrder := []string{
		"Number",
		"BrandName",
		"StartDatePeriode",
		"EndDatePeriode",
		"ActivityName",
		"Status",
		"CreatedBy",
		"CreatedDate",
		"TotalCosting",
		"CostingLama",
		"JnBalikkan",
		"Credit",
		"Total_Budget",
		"Balance",
		"DN_Dibuat",
		"DN_Dibayar",
		"CN_Principal",
		"GroupName",
		"Jml_SKP",
		"Kam",
		"Sumber",
		"KlaimKe",
		"FsYear",
		"BudgetType",
		"Toko",
		"NoDN",
		"BudgetActivity",
	}

	// Pass the constructed query, cache key, the desiredOrder slice, and parameters
	// to the GenericHtmlQueryHandler for caching and execution.
	return GenericHtmlQueryHandler(c, cacheKey, baseSQL, desiredOrder, params...)
}