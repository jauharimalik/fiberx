package handlers

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type KamProposalApprovedParams struct {
	Number      string   `query:"number"`
	UserCodeKam string   `query:"user_code_kam"`
	Brand       []string `query:"brand"`
	Group       []string `query:"group"`
	Activity    []string `query:"activity"`
	StartDate   string   `query:"start_date"`
	EndDate     string   `query:"end_date"`
	SKP         *int     `query:"skp"`
	Status      string   `query:"status"`
}

func Kamloadproposal(c *fiber.Ctx) error {
	params := new(KamProposalApprovedParams)
	if err := c.QueryParser(params); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(fmt.Sprintf("Error parsing query parameters: %v", err))
	}

	number := params.Number
	userCodeKam := params.UserCodeKam

	iskamall := 0
	if userCodeKam == "KA019" || userCodeKam == "KA006" {
		iskamall = 1
	}

	var sqlBuilder strings.Builder
	var conditions []string
	var paramsSQL []interface{}
	paramIndex := 1

	userInPicGroupAnp := true

	if userCodeKam == "KA029" {
		sqlBuilder.WriteString(fmt.Sprintf(`
			SELECT
			
			(
				select top 1 t4.GroupName from tb_proposal_customer t3
				left join m_group t4 on t3.GroupCustomer = t4.GroupCode
				where t0.[Number] = t3.ProposalNumber
			)as [Group],
				t0.id,
				t0.Number,
				(SELECT TOP 1 t1.BrandName FROM m_brand t1 WHERE t1.BrandCode = t0.brandcode) AS BrandName,
				t0.StartDatePeriode,
				t0.EndDatePeriode,
				(SELECT TOP 1 t1.promo_name FROM m_promo t1 WHERE t1.id = t0.Activity) AS ActivityName,
				t0.Status,
				t0.CreatedBy,
				t0.CreatedDate,
				(SELECT TOP 1 totalcosting FROM tb_operating_proposal t1 WHERE t1.ProposalNumber = t0.Number) AS TotalCosting,
				(SELECT COUNT(t1.id) FROM tb_proposal_group t1 WHERE t1.ProposalNumber = t0.Number) AS target_skp,
				(SELECT COUNT(t1.id) FROM tb_proposal_skp t1 WHERE t1.ProposalNumber = t0.Number AND t1.NoSKP != '') AS jml_skp
			FROM tb_proposal t0
			WHERE EXISTS (
				SELECT 1 FROM tb_operating_proposal t4
				INNER JOIN tb_proposal_customer t5 ON t5.ProposalNumber = t0.Number
				INNER JOIN [pk-query].[db_santosh].dbo.b_cust t6 ON t6.CardCode = t5.CustomerCode
				INNER JOIN master_user t7 ON t7.fullname = t6.rsm
				WHERE t7.user_code = @p%d AND t4.keperluan = 'internal'
			) AND t0.[Status] != 'canceled'
		`, paramIndex))
		paramsSQL = append(paramsSQL, userCodeKam)
		paramIndex++

		if number != "" {
			conditions = append(conditions, fmt.Sprintf("t0.Number = @p%d", paramIndex))
			paramsSQL = append(paramsSQL, number)
			paramIndex++
		}
	} else if userCodeKam == "KA032" {
		sqlBuilder.WriteString(fmt.Sprintf(`
			SELECT
			
			(
				select top 1 t4.GroupName from tb_proposal_customer t3
				left join m_group t4 on t3.GroupCustomer = t4.GroupCode
				where t0.[Number] = t3.ProposalNumber
			)as [Group],
				t0.id,
				t0.Number,
				(SELECT TOP 1 t1.BrandName FROM m_brand t1 WHERE t1.BrandCode = t0.brandcode) AS BrandName,
				t0.StartDatePeriode,
				t0.EndDatePeriode,
				(SELECT TOP 1 t1.promo_name FROM m_promo t1 WHERE t1.id = t0.Activity) AS ActivityName,
				t0.Status,
				t0.CreatedBy,
				t0.CreatedDate,
				(SELECT TOP 1 totalcosting FROM tb_operating_proposal t1 WHERE t1.ProposalNumber = t0.Number) AS TotalCosting,
				(SELECT COUNT(t1.id) FROM tb_proposal_group t1 WHERE t1.ProposalNumber = t0.Number) AS target_skp,
				(SELECT COUNT(t1.id) FROM tb_proposal_skp t1 WHERE t1.ProposalNumber = t0.Number AND t1.NoSKP != '') AS jml_skp
			FROM tb_proposal t0
			WHERE EXISTS (
				SELECT 1 FROM tb_operating_proposal t4
				INNER JOIN tb_proposal_customer t5 ON t5.ProposalNumber = t0.Number
				INNER JOIN [pk-query].[db_santosh].dbo.b_cust t6 ON t6.CardCode = t5.CustomerCode
				INNER JOIN master_user t7 ON t7.fullname = t6.rsm
				WHERE (t7.user_code = @p%d OR t6.rsm = 'Suyanto') AND t4.keperluan = 'internal'
			) AND t0.[Status] != 'canceled'
		`, paramIndex))
		paramsSQL = append(paramsSQL, userCodeKam)
		paramIndex++

		if number != "" {
			conditions = append(conditions, fmt.Sprintf("t0.Number = @p%d", paramIndex))
			paramsSQL = append(paramsSQL, number)
			paramIndex++
		}
	} else {
		sqlBuilder.WriteString(`
			SELECT
			
			(
				select top 1 t4.GroupName from tb_proposal_customer t3
				left join m_group t4 on t3.GroupCustomer = t4.GroupCode
				where t0.[Number] = t3.ProposalNumber
			)as [Group],
				t0.id,
				t0.Number,
				(SELECT TOP 1 t1.BrandName FROM m_brand t1 WHERE t1.BrandCode = t0.BrandCode) AS BrandName,
				t0.StartDatePeriode,
				t0.EndDatePeriode,
				(SELECT TOP 1 t1.promo_name FROM m_promo t1 WHERE t1.id = t0.Activity) AS ActivityName,
				t0.Status,
				t0.CreatedBy,
				t0.CreatedDate,
				(SELECT TOP 1 totalcosting FROM tb_operating_proposal t1 WHERE t1.ProposalNumber = t0.Number) AS TotalCosting,
				(SELECT COUNT(t1.id) FROM tb_proposal_group t1 WHERE t1.ProposalNumber = t0.Number) AS target_skp,
				(SELECT COUNT(t1.id) FROM tb_proposal_skp t1 WHERE t1.ProposalNumber = t0.Number AND t1.NoSKP != '') AS jml_skp
			FROM tb_proposal t0
			WHERE t0.[Status] != 'canceled'
		`)

		var existsConditions []string

		if userInPicGroupAnp {
			existsConditions = append(existsConditions, `t4.ProposalNumber = t0.Number AND t4.keperluan = 'internal'`)
			if iskamall == 0 {
				existsConditions = append(existsConditions, fmt.Sprintf("t7.user_code = @p%d", paramIndex))
				paramsSQL = append(paramsSQL, userCodeKam)
				paramIndex++
			}
		} else {
			existsConditions = append(existsConditions, `t4.ProposalNumber = t0.Number`)
			if iskamall == 0 {
				existsConditions = append(existsConditions, fmt.Sprintf("t7.user_code = @p%d AND (t6.SALES_HEAD = 'emma' OR t6.rsm = 'Aris')", paramIndex))
				paramsSQL = append(paramsSQL, userCodeKam)
				paramIndex++
			}
		}

		existsClause := fmt.Sprintf(`
			EXISTS (
				SELECT 1 FROM tb_operating_proposal t4
				LEFT JOIN tb_proposal_customer t5 ON t5.ProposalNumber = t0.Number
				LEFT JOIN [pk-query].db_santosh.dbo.b_cust t6 ON t6.CardCode = t5.CustomerCode
				LEFT JOIN master_user t7 ON t7.fullname = t6.KAM
				WHERE %s
			)`, strings.Join(existsConditions, " AND "))
		conditions = append(conditions, existsClause)

		if userCodeKam == "FN005" {
			conditions = append(conditions, `(t0.Activity = 49 OR EXISTS (
				SELECT 1 FROM tb_proposal_mechanism tm
				WHERE ((tm.Mechanism LIKE '%pom%')
				OR (tm.Mechanism LIKE '%regi%') OR (tm.Mechanism LIKE '%hala%') ) AND t0.Activity != 25 AND tm.ProposalNumber = t0.Number
			))`)
		}

		conditions = append(conditions, "t0.Activity != 29 AND t0.Activity != 30 AND t0.Activity != 39 AND t0.Activity != 31")
		conditions = append(conditions, "t0.StartDatePeriode >= '2023-12-31'")

		if number != "" {
			conditions = append(conditions, fmt.Sprintf("t0.Number = @p%d", paramIndex))
			paramsSQL = append(paramsSQL, number)
			paramIndex++
		}
	}

	if params.SKP != nil {
		switch *params.SKP {
		case 0:
			conditions = append(conditions, `t0.Number NOT IN (SELECT ProposalNumber FROM tb_proposal_skp)`)
		case 1:
			conditions = append(conditions, `EXISTS (SELECT 1 FROM tb_proposal_skp t_skp WHERE t_skp.ProposalNumber = t0.Number AND (t_skp.status_skp = '' OR t_skp.status_skp IS NULL))`)
		case 2:
			conditions = append(conditions, `EXISTS (SELECT 1 FROM tb_proposal_skp t_skp WHERE t_skp.ProposalNumber = t0.Number AND t_skp.status_skp = 'approve')`)
		case 3:
			conditions = append(conditions, `EXISTS (SELECT 1 FROM tb_proposal_skp t_skp WHERE t_skp.ProposalNumber = t0.Number AND t_skp.status_skp = 'canceled')`)
		}
	}

	if len(params.Brand) > 0 {
		conditions = append(conditions, fmt.Sprintf("t0.BrandCode IN (SELECT value FROM STRING_SPLIT(@p%d, ','))", paramIndex))
		paramsSQL = append(paramsSQL, strings.Join(params.Brand, ","))
		paramIndex++
	}

	if len(params.Group) > 0 {
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM tb_proposal_customer t5
			INNER JOIN m_group t6 ON t6.GroupCode = t5.GroupCustomer
			WHERE t5.ProposalNumber = t0.Number
			AND t6.GroupCode IN (SELECT value FROM STRING_SPLIT(@p%d, ','))
		)`, paramIndex))
		paramsSQL = append(paramsSQL, strings.Join(params.Group, ","))
		paramIndex++
	}

	if len(params.Activity) > 0 {
		conditions = append(conditions, fmt.Sprintf("t0.Activity IN (SELECT value FROM STRING_SPLIT(@p%d, ','))", paramIndex))
		paramsSQL = append(paramsSQL, strings.Join(params.Activity, ","))
		paramIndex++
	}

	if params.StartDate != "" {
		conditions = append(conditions, fmt.Sprintf("FORMAT(t0.StartDatePeriode, 'yyyyMMdd') >= FORMAT(CONVERT(DATE,@p%d), 'yyyyMMdd')", paramIndex))
		paramsSQL = append(paramsSQL, params.StartDate)
		paramIndex++
	} else {
		conditions = append(conditions, "(FORMAT(t0.StartDatePeriode, 'yyyyMMdd') >= '20250101' or FORMAT(t0.EndDatePeriode, 'yyyyMMdd') >= '20250101' or FORMAT(t0.CreatedDate, 'yyyyMMdd') >= '20250101')")
	}

	if params.EndDate != "" {
		conditions = append(conditions, fmt.Sprintf("FORMAT(t0.EndDatePeriode, 'yyyyMMdd') <= FORMAT(CONVERT(DATE,@p%d), 'yyyyMMdd')", paramIndex))
		paramsSQL = append(paramsSQL, params.EndDate)
		paramIndex++
	}

	if params.Status != "" {
		conditions = append(conditions, fmt.Sprintf("t0.Status = @p%d", paramIndex))
		paramsSQL = append(paramsSQL, params.Status)
		paramIndex++
	}

	if len(conditions) > 0 {
		if !strings.Contains(sqlBuilder.String(), "WHERE") {
			sqlBuilder.WriteString(" WHERE ")
		} else {
			sqlBuilder.WriteString(" AND ")
		}
		sqlBuilder.WriteString(strings.Join(conditions, " AND "))
	}

	sqlBuilder.WriteString(" ORDER BY t0.CreatedDate DESC")
	finalSQL := sqlBuilder.String()

	fmt.Println("Generated SQL:", finalSQL)
	fmt.Println("SQL Parameters:", paramsSQL)

	return GenericQueryHandler(c, finalSQL, paramsSQL...)
}