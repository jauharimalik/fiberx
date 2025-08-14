package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors" // Import CORS middleware
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	html "github.com/gofiber/template/html/v2"
	"my-fiber-app/db"      // Import package db
	"my-fiber-app/handlers" // Import package handlers
)

func main() {
	// Inisialisasi template engine
	engine := html.New("./templates", ".html")
	app := fiber.New(fiber.Config{
		Views: engine,
	})

	// Tambahkan middleware CORS di sini
	// Ini akan mengizinkan permintaan dari origin manapun (*).
	// Untuk produksi, Anda sebaiknya membatasi ini ke origin frontend Anda yang sebenarnya.
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*", // Ganti dengan origin frontend Anda, contoh: "http://192.168.60.19:4245:3000"
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	// Middleware untuk menyajikan file statis dari direktori 'assets'
	app.Use("/assets", filesystem.New(filesystem.Config{
		Root:   http.Dir("./assets"),
		Browse: false,
	}))

	// Middleware untuk menyajikan file statis dari direktori 'wms/assets'
	app.Use("/wms/assets", filesystem.New(filesystem.Config{
		Root:   http.Dir("./wms/assets"),
		Browse: false,
	}))

	// Koneksi ke database (panggil hanya sekali saat aplikasi dimulai)
	db.Connect()
	// Pastikan koneksi database ditutup saat aplikasi berhenti
	defer db.CloseDB()

	// go handlers.StartLatLonUpdater()
	// go handlers.StartJarakUpdater()
	go handlers.Cekplat()
	go handlers.RunDeleteSkpTask()
	go handlers.KirimUltah()
	go handlers.KirimKontrak()
	go handlers.StartDailyExcelToJsonUpdater()

	// Definisi rute-rute aplikasi
	// Asumsi Demo1Handler dan Gettokenarobs ada di package handlers dan sudah didefinisikan
	app.Get("/demo1", handlers.Demo1Handler)
	app.Get("/pkexpress/gettokenarobs", handlers.Gettokenarobs)
	app.Get("/pkexpress/kasusplat", handlers.KasusPlatHandler)
	
	app.Get("/pkexpress/addrlatlon/:addr", handlers.NewAddrlatlon)
	// app.Get("/api/save-karyawan-excel-to-json", handlers.SaveKaryawanExcelToJson)
	// --- Akhir penambahan rute ---
	app.Get("/contoh", func(c *fiber.Ctx) error {
		anu := c.Query("anu")
		return c.SendString("Isi dari parameter 'anu' adalah: " + anu)
	})

	// Rute untuk Demo2Handler dengan parameter opsional
	// Perhatikan urutan rute: yang lebih spesifik harus di atas
	
	app.Get("/anp/apixl", handlers.AnpApiXLHandler)
	app.Get("/anp/budgetsisa", handlers.AnpBudgetSisaHandler)
	app.Get("/anp/loadtabelproposal", handlers.Loadtabelproposal)
	app.Get("/anp/dirloadtableproposal", handlers.Dirloadtableproposal)
	app.Get("/anp/kamloadtableproposal", handlers.Kamloadproposal)
	
	
	app.Get("/hr/SendWaJs2", handlers.SendWaJs2)
	app.Get("/hr/UpdateKontrak", handlers.UpdateKontrak)
	app.Get("/hr/SendBirthdayWishes", handlers.SendBirthdayWishes)
	app.Get("/hr/kirimkontrak", handlers.GenerateKirimKontrakHTML)
	app.Get("/hr/kirimultah", handlers.GenerateKirimUltahHTML)
	

	app.Get("/pos/customerload", handlers.CustomerLoad)
	app.Get("/pos/approvepo", handlers.ApproveOrderHandler)
	app.Get("/pos/orderlistdata", handlers.GetOrderDataHandler)
	
	

	app.Get("/wms/cekitem/:itemCode/:whscode/:exp/:binl", handlers.Demo2Handler)
	app.Get("/wms/cekitem/:itemCode/:whscode/:exp", handlers.Demo2Handler)
	app.Get("/wms/cekitem/:itemCode/:whscode", handlers.Demo2Handler)
	app.Get("/wms/cekitem/:itemCode", handlers.Demo2Handler)
	
	// Rute untuk DataTables server-side (POST request)
	// Menggunakan AjaxManifesHandler dari package handlers
	app.Get("/pkexpress/ajaxmanifes", handlers.AjaxManifesHandler)
	app.Get("/pkexpress/jarak", handlers.KonversijarakHandler)
	
	app.Get("/pkexpress/konversialamat", handlers.MapboxGeocodeHandler)
	

	// Rute untuk menyajikan file HTML dinamis dari direktori 'templates'
	app.Get("/wms/:filename", func(c *fiber.Ctx) error {
		filename := c.Params("filename")
		filePath := filepath.Join("./templates", filename+".html") // Gabungkan path direktori dan nama file

		// Periksa apakah file ada
		_, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			// Jika file tidak ada, kembalikan 404
			return c.Status(fiber.StatusNotFound).SendString("File not found")
		} else if err != nil {
			// Jika ada kesalahan lain saat memeriksa file, kembalikan 500
			log.Printf("Error checking file: %v", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
		}

		// Jika file ada, kirimkan sebagai respons
		return c.SendFile(filePath)
	})

	// Rute untuk merender template 'scan.html'
	app.Get("/scan", func(c *fiber.Ctx) error {
		return c.Render("scan", fiber.Map{})
	})

	// Mulai server Fiber pada port 80
	log.Fatal(app.Listen(":4245"))
}
