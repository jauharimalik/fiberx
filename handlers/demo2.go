package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// Demo2Handler mengambil itemcode dari URL dan menjalankan query untuk mengambil detail item dari database.
func Demo2Handler(c *fiber.Ctx) error {
	itemcode := c.Params("itemcode") 
	whscode := c.Params("whscode") 
	exp := c.Params("exp") 
	// binl := c.Params("binl") 
	query := `
		SELECT
			BRAND,
			itemcode,
			FrgnName,
			ItemName,
			WhsCode,
			CAST(ExpDate AS DATE) AS ExpDate,
			SUM(ISNULL(Quantity, 0)) AS Quantity,
			SUM(ISNULL(Quantity, 0) / NumInBuy) AS Qty_CRT,
			SUM(ISNULL(Commited, 0)) AS Commited,
			SUM(ISNULL(Commited, 0) / NumInBuy) AS Commited_CRT,
			CONVERT(VARCHAR, GETDATE(), 106) AS Cutoff_Day,
			DATEDIFF(Day, GETDATE(), ExpDate) AS Aging
		FROM (
			SELECT
				DISTINCT E.ItmsGrpNam AS BRAND,
				C.itemcode,
				C.FrgnName,
				C.NumInBuy,
				C.ItemName,
				B.WhsCode,
				CAST(D.ExpDate AS DATE) AS ExpDate,
				ISNULL(A_1.Quantity, 0) AS Quantity,
				ISNULL(A_1.IsCommited, 0) AS Commited
			FROM (
				SELECT
					itemcode,
					BatchNum,
					WhsCode,
					ItemName,
					SuppSerial,
					IntrSerial,
					ExpDate,
					PrdDate,
					InDate,
					Located,
					Notes,
					Quantity,
					BaseType,
					BaseEntry,
					BaseNum,
					BaseLinNum,
					CardCode,
					CardName,
					CreateDate,
					Status,
					Direction,
					IsCommited,
					OnOrder,
					Consig,
					DataSource,
					UserSign,
					Transfered,
					Instance,
					SysNumber,
					LogInstanc,
					UserSign2,
					UpdateDate,
					U_Panjang2,
					U_Berat,
					U_Lebar,
					U_Panjang,
					U_Group,
					U_Shift,
					U_Jenis,
					U_Micron,
					U_Mesin,
					U_IDU_PalletID
				FROM
					[pksrv-sap].[pandurasa_live].dbo.OIBT AS A WITH (NOLOCK)
				WHERE
					(Quantity <> 0)
					AND ((itemcode + WhsCode) IN (
						SELECT
							DISTINCT itemcode + WhsCode AS Expr1
						FROM
							[pksrv-sap].[pandurasa_live].dbo.OITW WITH (NOLOCK)
						WHERE
							(OnHand <> 0)
					))
			) AS A_1
			INNER JOIN [pksrv-sap].[pandurasa_live].dbo.OITW AS B WITH (NOLOCK) ON A_1.itemcode = B.itemcode
				AND A_1.WhsCode = B.WhsCode
			INNER JOIN [pksrv-sap].[pandurasa_live].dbo.OITM AS C WITH (NOLOCK) ON B.itemcode = C.itemcode
			INNER JOIN [pksrv-sap].[pandurasa_live].dbo.OBTN AS D WITH (NOLOCK) ON A_1.BatchNum = D.DistNumber
				AND A_1.itemcode = D.itemcode
			INNER JOIN [pksrv-sap].[pandurasa_live].dbo.OITB AS E WITH (NOLOCK) ON E.ItmsGrpCod = C.ItmsGrpCod
			WHERE
				(C.validFor = 'Y')
				AND (C.FrgnName IS NOT NULL)
				AND (A_1.Quantity <> 0)
				AND (C.itemcode = @itemcode)
		) AS XX
		WHERE
			XX.itemcode = @itemcode
			`

	queryParams := []interface{}{itemcode}

	if whscode != "" {
		query += `AND XX.WhsCode = @whscode`
		queryParams = append(queryParams, whscode)
	}

	if exp != "" {
		query += `AND XX.ExpDate = @exp`
		queryParams = append(queryParams, exp)
	}

	// if binl != "" {
	// 	query += `
	// 		AND XX.WhsCode = @binl
	// 	`
	// 	queryParams = append(queryParams, binl)
	// }


	query += `
		GROUP BY
			BRAND,
			itemcode,
			FrgnName,
			ItemName,
			WhsCode,
			ExpDate
		ORDER BY BRAND, itemcode;
	`
	return GenericQueryHandler(c, query, queryParams...)
}