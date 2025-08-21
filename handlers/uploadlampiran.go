package handlers

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"my-fiber-app/db" // Kita akan menggunakan koneksi dari sini
)

// Struktur yang merepresentasikan data dari query SQL
type Proposal struct {
	Number           string
	BrandName        string
	PromoName        string
	StartDatePeriode string
	EndDatePeriode   string
	UserCode         string
	Status           string
	Username         string
	WaKam            sql.NullString
	WaNkam           sql.NullString
	WaPm             sql.NullString
	WaLeaderPm       sql.NullString
	EmailKam         sql.NullString
	EmailNkam        sql.NullString
	EmailPm          sql.NullString
	EmailLeaderPm    sql.NullString
}

// Struktur untuk data dari tabel kirimapp
type KirimAppUser struct {
	NomorWhatsapp sql.NullString
	Email         sql.NullString
}

// Konfigurasi aplikasi
const (
	SourceDir      = "lampiran"
	DestinationDir = "uploads/img/att"
	baseURL        = "http://your-base-url.com" // Ganti dengan URL dasar yang sesuai
)

// sendNotification mengirim pesan melalui HTTP, menggantikan AJAX di PHP.
func sendNotification(apiURL, messages, recipient, filename string) {
	// Membentuk URL dengan parameter query
	params := url.Values{}
	params.Add("messages", messages)
	params.Add("no", recipient)
	params.Add("namafile", filename)

	fullURL := fmt.Sprintf("%s/%s.php?%s", baseURL, apiURL, params.Encode())

	// Melakukan GET request ke API
	resp, err := http.Get(fullURL)
	if err != nil {
		log.Printf("Gagal mengirim notifikasi ke %s: %v", fullURL, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Panggilan API gagal untuk %s dengan status: %s", fullURL, resp.Status)
	} else {
		log.Printf("Notifikasi berhasil dikirim ke %s", recipient)
	}
}

// uploadMassalLampiran melakukan proses pemindahan file dan update database
func UploadMassalLampiran() {
	if db.DB == nil {
		log.Println("Koneksi database tidak tersedia.")
		return
	}

	filesAndDirs, err := readDirectory(SourceDir)
	if err != nil {
		log.Printf("Gagal membaca direktori: %v", err)
		return
	}

	if _, err := os.Stat(DestinationDir); os.IsNotExist(err) {
		os.MkdirAll(DestinationDir, os.ModePerm)
	}

	for _, filePath := range filesAndDirs {
		filename := filepath.Base(filePath)
		if filename == "" {
			continue
		}

		filenya := strings.TrimSuffix(filename, filepath.Ext(filename))
		parts := strings.Split(filenya, "-")
		if len(parts) == 0 {
			continue
		}

		nomorx := strings.ToLower(strings.TrimSpace(parts[0]))
		nomorx = strings.ReplaceAll(nomorx, " ", "")
		nomorx = strings.ReplaceAll(nomorx, "pk", "")

		query := `
		SELECT TOP(1)
			t0.number, t2.BrandName, t1.promo_name, t0.StartDatePeriode, t0.EndDatePeriode,
			t0.UserCode, t0.Status,
			(SELECT TOP(1) username FROM master_user WHERE user_code = t0.UserCode) AS username,
			(SELECT TOP(1) whatsapp FROM tb_proposal_customer AS tx0
				INNER JOIN b_cust AS tx2 ON tx0.CustomerCode = tx2.CardCode
				INNER JOIN [SERVICE-DESK].dbo.master_users AS tx4 ON tx4.inisial LIKE '%' + tx2.kam + '%'
				WHERE tx0.ProposalNumber = t0.Number) AS wa_kam,
			(SELECT TOP(1) whatsapp FROM tb_proposal_customer AS tx0
				INNER JOIN b_cust AS tx2 ON tx0.CustomerCode = tx2.CardCode
				INNER JOIN [SERVICE-DESK].dbo.master_users AS tx4 ON tx4.inisial LIKE '%' + tx2.SALES_HEAD + '%'
				WHERE tx0.ProposalNumber = t0.Number) AS wa_nkam,
			(SELECT TOP(1) whatsapp FROM master_user AS tx0
				INNER JOIN [SERVICE-DESK].dbo.master_users AS tx4 ON tx4.inisial LIKE '%' + tx0.fullname + '%'
				WHERE tx0.user_code = t0.UserCode) AS wa_pm,
			(SELECT TOP(1) whatsapp FROM tb_proposal_customer AS tx0
				INNER JOIN b_cust AS tx2 ON tx0.CustomerCode = tx2.CardCode
				INNER JOIN [SERVICE-DESK].dbo.master_users AS tx4 ON tx4.inisial LIKE '%' + tx2.leader + '%'
				WHERE tx0.ProposalNumber = t0.Number) AS wa_leaderpm,
			(SELECT TOP(1) email FROM tb_proposal_customer AS tx0
				INNER JOIN b_cust AS tx2 ON tx0.CustomerCode = tx2.CardCode
				INNER JOIN [SERVICE-DESK].dbo.master_users AS tx4 ON tx4.inisial LIKE '%' + tx2.kam + '%'
				WHERE tx0.ProposalNumber = t0.Number) AS email_kam,
			(SELECT TOP(1) email FROM tb_proposal_customer AS tx0
				INNER JOIN b_cust AS tx2 ON tx0.CustomerCode = tx2.CardCode
				INNER JOIN [SERVICE-DESK].dbo.master_users AS tx4 ON tx4.inisial LIKE '%' + tx2.SALES_HEAD + '%'
				WHERE tx0.ProposalNumber = t0.Number) AS email_nkam,
			(SELECT TOP(1) email FROM master_user AS tx0
				INNER JOIN [SERVICE-DESK].dbo.master_users AS tx4 ON tx4.inisial LIKE '%' + tx0.fullname + '%'
				WHERE tx0.user_code = t0.UserCode) AS email_pm,
			(SELECT TOP(1) email FROM tb_proposal_customer AS tx0
				INNER JOIN b_cust AS tx2 ON tx0.CustomerCode = tx2.CardCode
				INNER JOIN [SERVICE-DESK].dbo.master_users AS tx4 ON tx4.inisial LIKE '%' + tx2.leader + '%'
				WHERE tx0.ProposalNumber = t0.Number) AS email_leaderpm
		FROM tb_proposal AS t0
		INNER JOIN m_promo AS t1 ON t0.Activity = t1.id
		INNER JOIN m_brand AS t2 ON t0.BrandCode = t2.BrandCode
		WHERE t0.Number LIKE @p1
		`
		var proposal Proposal
		err = db.DB.QueryRow(query, "%"+nomorx+"%").Scan(
			&proposal.Number, &proposal.BrandName, &proposal.PromoName, &proposal.StartDatePeriode, &proposal.EndDatePeriode,
			&proposal.UserCode, &proposal.Status, &proposal.Username, &proposal.WaKam, &proposal.WaNkam, &proposal.WaPm,
			&proposal.WaLeaderPm, &proposal.EmailKam, &proposal.EmailNkam, &proposal.EmailPm, &proposal.EmailLeaderPm,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				log.Printf("Tidak ada proposal yang ditemukan untuk file: %s", filename)
			} else {
				log.Printf("Gagal menjalankan query: %v", err)
			}
			continue
		}

		destinationPath := filepath.Join(DestinationDir, filename)
		err = os.Rename(filePath, destinationPath)
		if err != nil {
			log.Printf("Gagal memindahkan file: %v", err)
			continue
		}

		log.Printf("File '%s' berhasil dipindahkan ke '%s'", filename, destinationPath)

		// Membangun string pesan
		var text bytes.Buffer
		text.WriteString("UPLOAD LAMPIRAN\n")
		text.WriteString("========================================================\n")
		text.WriteString(fmt.Sprintf("Nomor : %s\n", proposal.Number))
		text.WriteString(fmt.Sprintf("Nama Merek : %s\n", proposal.BrandName))
		text.WriteString(fmt.Sprintf("Nama Promo : %s\n", proposal.PromoName))
		text.WriteString(fmt.Sprintf("Tanggal Mulai Periode : %s\n", proposal.StartDatePeriode))
		text.WriteString(fmt.Sprintf("Tanggal Akhir Periode : %s\n", proposal.EndDatePeriode))
		text.WriteString("========================================================\n")

		// Mengirim notifikasi ke berbagai pihak
		if proposal.WaKam.Valid {
			sendNotification("kirimwa", text.String(), proposal.WaKam.String, destinationPath)
		}
		if proposal.EmailKam.Valid {
			sendNotification("mailer", text.String(), proposal.EmailKam.String, destinationPath)
		}
		if proposal.WaNkam.Valid {
			sendNotification("kirimwa", text.String(), proposal.WaNkam.String, destinationPath)
		}
		if proposal.EmailNkam.Valid {
			sendNotification("mailer", text.String(), proposal.EmailNkam.String, destinationPath)
		}
		if proposal.WaPm.Valid {
			sendNotification("kirimwa", text.String(), proposal.WaPm.String, destinationPath)
		}
		if proposal.EmailPm.Valid {
			sendNotification("mailer", text.String(), proposal.EmailPm.String, destinationPath)
		}

		rows, err := db.DB.Query("SELECT nomor_whatsapp, email FROM kirimapp WHERE is_active = 'y'")
		if err != nil {
			log.Printf("Gagal mendapatkan user kirimapp: %v", err)
			continue
		}
		defer rows.Close()

		for rows.Next() {
			var appUser KirimAppUser
			if err := rows.Scan(&appUser.NomorWhatsapp, &appUser.Email); err != nil {
				log.Printf("Gagal memindai baris kirimapp: %v", err)
				continue
			}
			if appUser.NomorWhatsapp.Valid {
				sendNotification("kirimwa", text.String(), appUser.NomorWhatsapp.String, destinationPath)
			}
			if appUser.Email.Valid {
				sendNotification("mailer", text.String(), appUser.Email.String, destinationPath)
			}
		}

		// Menghapus data lampiran yang lama dan menyisipkan yang baru
		deleteQuery := `DELETE FROM tb_proposal_lampiran WHERE proposalnumber=@p1 AND filex=@p2`
		_, err = db.DB.Exec(deleteQuery, proposal.Number, filepath.Base(destinationPath))
		if err != nil {
			log.Printf("Gagal menghapus entri lampiran: %v", err)
			continue
		}

		insertQuery := `
		INSERT INTO tb_proposal_lampiran (proposalnumber, filex, note, created_by, created_at)
		VALUES (@p1, @p2, @p3, @p4, @p5)
		`
		_, err = db.DB.Exec(insertQuery, proposal.Number, filepath.Base(destinationPath), fmt.Sprintf("%s - Upload Via Scanner", proposal.Number), "fin", time.Now())
		if err != nil {
			log.Printf("Gagal menyisipkan entri lampiran: %v", err)
			continue
		}
	}

	updateQuery := `UPDATE tb_proposal_lampiran SET filex = REPLACE(filex, 'uploads/img/att/', '')`
	_, err = db.DB.Exec(updateQuery)
	if err != nil {
		log.Printf("Gagal memperbarui path file di database: %v", err)
	}
}

// readDirectory membaca semua file dalam sebuah direktori
func readDirectory(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil { return err }
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// RunUploadLampiranAuto adalah fungsi yang akan dijalankan sebagai background task
func RunUploadLampiranAuto() {
	for {
		log.Println("Memulai proses upload lampiran...")
		UploadMassalLampiran()
		log.Println("Proses upload selesai. Menunggu 20 detik untuk mengulangi.")
		time.Sleep(20 * time.Second)
	}
}