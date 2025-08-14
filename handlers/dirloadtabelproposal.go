package handlers

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type ProposalApprovedParams struct {
	Number    string   `query:"number"`
	Brand     []string `query:"brand"`
	Group     []string `query:"group"`
	Activity  []string `query:"activity"`
	StartDate string   `query:"start_date"`
	EndDate   string   `query:"end_date"`
	SKP       *int     `query:"skp"`
	Status    string   `query:"status"`
}

// Dirloadtableproposal is now exported
func Dirloadtableproposal(c *fiber.Ctx) error {
	params := new(ProposalApprovedParams)
	if err := c.QueryParser(params); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(fmt.Sprintf("Error parsing query parameters: %v", err))
	}

	var sqlBuilder strings.Builder
	sqlBuilder.WriteString(`
		SELECT DISTINCT
			t1.id,
			t1.Number,
			mb.BrandName,
			ISNULL(top_prop.jnbalikkan, t1.noref )as Reff,
			t1.StartDatePeriode,
			t1.EndDatePeriode,
			mp.promo_name AS ActivityName,
			t1.Status,
			t1.CreatedBy,
			isnull(tbp.Pic,'Management') as Pic,
			(
				select top 1 tbpx.Pic from tb_pic_brand tbpx
				where tbpx.BrandCode = t1.BrandCode and tbpx.Pic not like '%regis%' and tbpx.levelx is not null
				order by tbpx.levelx desc
			) as user_created,
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
			t1.CreatedDate,
			t1.ClaimTo,
			ISNULL(top_prop.Realisasi, 0) AS Realisasi,
			UPPER(
				CASE
					WHEN top_prop.klaimable = 'y' AND NOT EXISTS (SELECT 1 FROM tb_proposal_dn tdn WHERE tdn.proposalnumber = t1.number) THEN 'yn'
					WHEN top_prop.klaimable = 'y' AND EXISTS (SELECT 1 FROM tb_proposal_dn tdn WHERE tdn.proposalnumber = t1.number) THEN 'y'
					WHEN top_prop.klaimable = 'n' AND EXISTS (SELECT 1 FROM tb_proposal_dn tdn WHERE tdn.proposalnumber = t1.number) THEN 'n'
					ELSE ISNULL(top_prop.klaimable, '0')
				END
			) AS klaimable,
			ISNULL(top_prop.ClaimTo, 0) AS ClaimTo,
			top_prop.Budget_type,
			ISNULL(top_prop.costing_lama, 0) AS TotalCosting,
			ISNULL(top_prop.TotalCosting, 0) AS TotalCosting2,
			ISNULL(cn_pot.Credit, 0) AS Creditx,
			ISNULL(top_prop.Realisasi, 0) AS Credit,
			
			isnull(
				(
					select top 1 CONCAT(tpa.username, ' - ', CONVERT(VARCHAR(10), tpa.created_at, 23))
					from tb_proposal_approved tpa 
					where tpa.proposalnumber =  t1.number order by tpa.id desc)
			,'') as Management,
			(
				select top 1 t4.GroupName from tb_proposal_customer t3
				left join m_group t4 on t3.GroupCustomer = t4.GroupCode
				where t1.[Number] = t3.ProposalNumber
			)as [Group],
			LEFT(top_prop.jnbalikkan, 20) as jnbalikkan,
			(
				SELECT DISTINCT TOP 1 t6.GroupName
				FROM tb_proposal_customer t5
				INNER JOIN m_group t6 ON t6.GroupCode = t5.GroupCustomer
				WHERE t5.ProposalNumber = t1.Number
			) AS GroupName,
			(SELECT COUNT(id) FROM tb_proposal_skp WHERE ProposalNumber = t1.Number AND NoSKP != '') AS jml_skp,
			(SELECT COUNT(id) FROM tb_proposal_lampiran WHERE ProposalNumber = t1.Number ) AS lampiran
		FROM
			tb_proposal t1
		LEFT JOIN m_brand mb ON mb.BrandCode = t1.BrandCode
		left join tb_pic_brand tbp on t1.BrandCode =tbp.BrandCode and (tbp.levelx is null or tbp.levelx <= 1)
		LEFT JOIN m_promo mp ON mp.id = t1.Activity
		OUTER APPLY (
			SELECT TOP 1
				klaimable,
				ClaimTo,
				costing_lama,
				TotalCosting,
				Budget_type,
				Realisasi,
				jnbalikkan
			FROM tb_operating_proposal
			WHERE ProposalNumber = t1.Number
		) top_prop
		LEFT JOIN (
			SELECT TOP 1
				Credit,
				u_idu_noproposal
			FROM tb_anp_cn_potongan
		) cn_pot ON cn_pot.u_idu_noproposal = t1.Number
	`)

	var conditions []string
	var paramsSQL []interface{}
	paramIndex := 1

	if params.SKP != nil {
		switch *params.SKP {
		case 0:
			sqlBuilder.WriteString(" LEFT JOIN tb_proposal_skp t1x ON t1.Number = t1x.ProposalNumber AND t1x.ProposalNumber IS NULL ")
		case 1:
			sqlBuilder.WriteString(" INNER JOIN tb_proposal_skp t1x ON t1.Number = t1x.ProposalNumber AND (t1x.status_skp = '' OR t1x.status_skp IS NULL) ")
		case 2:
			sqlBuilder.WriteString(" INNER JOIN tb_proposal_skp t1x ON t1.Number = t1x.ProposalNumber AND t1x.status_skp = 'approve' ")
		case 3:
			sqlBuilder.WriteString(" INNER JOIN tb_proposal_skp t1x ON t1.Number = t1x.ProposalNumber AND t1x.status_skp = 'canceled' ")
		}
	}

	conditions = append(conditions, "t1.[Status] != ''")

	if params.Number != "" {
		conditions = append(conditions, fmt.Sprintf("t1.Number = @p%d", paramIndex))
		paramsSQL = append(paramsSQL, params.Number)
		paramIndex++
	}

	if len(params.Brand) > 0 {
		conditions = append(conditions, fmt.Sprintf("t1.BrandCode IN (SELECT value FROM STRING_SPLIT(@p%d, ','))", paramIndex))
		paramsSQL = append(paramsSQL, strings.Join(params.Brand, ","))
		paramIndex++
	}

	if len(params.Group) > 0 {
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM tb_proposal_customer t5
			INNER JOIN m_group t6 ON t6.GroupCode = t5.GroupCustomer
			WHERE t5.ProposalNumber = t1.Number
			AND t6.GroupCode IN (SELECT value FROM STRING_SPLIT(@p%d, ','))
		)`, paramIndex))
		paramsSQL = append(paramsSQL, strings.Join(params.Group, ","))
		paramIndex++
	}

	if len(params.Activity) > 0 {
		conditions = append(conditions, fmt.Sprintf("t1.Activity IN (SELECT value FROM STRING_SPLIT(@p%d, ','))", paramIndex))
		paramsSQL = append(paramsSQL, strings.Join(params.Activity, ","))
		paramIndex++
	}

	if params.StartDate != "" {
		conditions = append(conditions, fmt.Sprintf("FORMAT(t1.StartDatePeriode, 'yyyyMMdd') >= FORMAT(CONVERT(DATE,@p%d), 'yyyyMMdd')", paramIndex))
		paramsSQL = append(paramsSQL, params.StartDate)
		paramIndex++
	} else {
		conditions = append(conditions, "FORMAT(t1.StartDatePeriode, 'yyyyMMdd') >= '20250101'")
	}

	if params.EndDate != "" {
		conditions = append(conditions, fmt.Sprintf("FORMAT(t1.EndDatePeriode, 'yyyyMMdd') <= FORMAT(CONVERT(DATE,@p%d), 'yyyyMMdd')", paramIndex))
		paramsSQL = append(paramsSQL, params.EndDate)
		paramIndex++
	}

	if params.Status != "" {
		conditions = append(conditions, fmt.Sprintf("t1.status = @p%d", paramIndex))
		paramsSQL = append(paramsSQL, params.Status)
		paramIndex++
	}

	
	if len(conditions) > 0 {
		sqlBuilder.WriteString(" WHERE " + strings.Join(conditions, " AND "))
	}

	sqlBuilder.WriteString(" ORDER BY t1.CreatedDate DESC")
	finalSQL := sqlBuilder.String()
	return GenericQueryHandler(c, finalSQL, paramsSQL...)
}