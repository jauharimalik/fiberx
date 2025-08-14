package handlers

import (
	"fmt"
	"net/http"
	"sync"
	"time"
	"strings"
	
	"github.com/gofiber/fiber/v2"
	"github.com/robfig/cron/v3"
)

// visitURL makes an HTTP GET request to the given URL.
// This function remains the same, used by both birthday and contract notifications.
func visitURL(url string) {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Error visiting URL %s: %v\n", url, err)
		return
	}
	defer resp.Body.Close()
	fmt.Printf("Visited URL: %s, Status: %s\n", url, resp.Status)
}

// --- Fungsi untuk Notifikasi Ulang Tahun (Sudah Ada) ---
// KirimUltah executes the specified URLs concurrently for birthday notifications.
func KirimUltah() {
	urls := []string{
		"http://192.168.60.19:4245/hr/SendBirthdayWishes?no=08118881895",
		"http://192.168.60.19:4245/hr/SendBirthdayWishes?no=087842300751",
		"http://192.168.60.19:4245/hr/SendBirthdayWishes?no=085781550337",
		"http://192.168.60.19:4245/hr/SendBirthdayWishes?no=081211747667",
	}

	var wg sync.WaitGroup
	for _, url := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			visitURL(u)
		}(url)
	}
	wg.Wait()
	fmt.Println("All birthday notifications sent.")
}

// --- Fungsi Baru untuk Notifikasi Kontrak ---
// KirimKontrak executes the specified URLs concurrently for contract updates.
func KirimKontrak() {
	urls := []string{
		"http://192.168.60.19:4245/hr/UpdateKontrak?no=087842300751",
		"http://192.168.60.19:4245/hr/UpdateKontrak?no=085781550337",
		"http://192.168.60.19:4245/hr/UpdateKontrak?no=081211747667",
	}

	var wg sync.WaitGroup
	for _, url := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			visitURL(u)
		}(url)
	}
	wg.Wait()
	fmt.Println("All contract update notifications sent.")
}

func GenerateKirimKontrakHTML(c *fiber.Ctx) error {
	numbers := []string{
		"087842300751",
		"085781550337",
		"081211747667",
	}

	var visitCalls strings.Builder
	for _, num := range numbers {
		visitCalls.WriteString(fmt.Sprintf("visitUrl('http://192.168.60.19:4245/hr/UpdateKontrak?no=%s');\n", num))
	}

	jsScript  := fmt.Sprintf(`
		<script>
		async function visitUrl(url) {
			try {
				let response = await fetch(url);
				let result = await response.text();
			} catch (error) {}
		};
		%s</script>`, visitCalls.String())


		c.Set("Content-Type", "text/html")
		return c.SendString(jsScript)
}

// GenerateKirimUltahHTML menghasilkan string HTML berisi JavaScript untuk memanggil endpoint ultahnotif
func GenerateKirimUltahHTML(c *fiber.Ctx) error {
	numbers := []string{
		"08118881895",
		"087842300751",
		"085781550337",
		"081211747667",
	}

	var visitCalls strings.Builder
	for _, num := range numbers {
		visitCalls.WriteString(fmt.Sprintf("visitUrl('http://192.168.60.19:4245/hr/SendBirthdayWishes?no=%s');\n", num))
	}

	jsScript := fmt.Sprintf(`
		<script>
		async function visitUrl(url) {
			try {
				let response = await fetch(url);
				let result = await response.text();
			} catch (error) {}
		};
		%s</script>`, visitCalls.String())

		c.Set("Content-Type", "text/html")
		return c.SendString(jsScript)
}


// StartScheduler initializes and starts all cron jobs.
// This function should be called from your main.go.
// func StartScheduler() {
// 	fmt.Println("Starting application scheduler...")

// 	c := cron.New()

// 	// --- Penjadwalan Notifikasi Ulang Tahun (Sudah Ada) ---
// 	// Schedule the KirimUltah function to run every day at 06:00
// 	_, errUltah := c.AddFunc("0 6 * * *", func() {
// 		fmt.Printf("Executing KirimUltah at %s\n", time.Now().Format("2006-01-02 15:04:05"))
// 		KirimUltah()
// 	})
// 	if errUltah != nil {
// 		fmt.Printf("Error scheduling birthday cron job: %v\n", errUltah)
// 		return
// 	}

// 	// --- Penjadwalan Notifikasi Kontrak (BARU) ---
// 	// Anda perlu menentukan jadwal yang diinginkan di sini.
// 	// Contoh: Setiap hari pukul 07:00 pagi.
// 	// Atau Anda bisa menggunakan interval lain, misalnya setiap jam, setiap menit, dll.
// 	_, errKontrak := c.AddFunc("0 7 * * *", func() { // Contoh: Setiap hari pukul 07:00 pagi
// 		fmt.Printf("Executing KirimKontrak at %s\n", time.Now().Format("2006-01-02 15:04:05"))
// 		KirimKontrak()
// 	})
// 	if errKontrak != nil {
// 		fmt.Printf("Error scheduling contract cron job: %v\n", errKontrak)
// 		return
// 	}


// 	c.Start()
// 	fmt.Println("Application scheduler started successfully.")
// }

func StartScheduler() {
	fmt.Println("Starting application scheduler...")

	c := cron.New()
	fmt.Printf("Current time: %v\n", time.Now()) 
	// --- Penjadwalan Notifikasi Ulang Tahun ---
	// Schedule the KirimUltah function to run every day at 10:00 (10 AM)
	_, errUltah := c.AddFunc("0 10 * * *", func() { // <--- Changed from "0 6 * * *" to "0 10 * * *"
		fmt.Printf("Executing KirimUltah at %s\n", time.Now().Format("2006-01-02 15:04:05"))
		KirimUltah()
	})
	if errUltah != nil {
		fmt.Printf("Error scheduling birthday cron job: %v\n", errUltah)
		return
	}

	// --- Penjadwalan Notifikasi Kontrak ---
	// Schedule the KirimKontrak function to run every day at 10:00 (10 AM)
	_, errKontrak := c.AddFunc("0 10 * * *", func() { // <--- Changed from "0 7 * * *" to "0 10 * * *"
		fmt.Printf("Executing KirimKontrak at %s\n", time.Now().Format("2006-01-02 15:04:05"))
		KirimKontrak()
	})
	if errKontrak != nil {
		fmt.Printf("Error scheduling contract cron job: %v\n", errKontrak)
		return
	}

	c.Start()
	fmt.Println("Application scheduler started successfully.")
}

