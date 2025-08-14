package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Mapbox API Access Token (Ganti dengan token Anda jika ini bukan contoh)
// Penting: Untuk aplikasi produksi, jangan menyimpan token API langsung di kode.
// Gunakan variabel lingkungan atau sistem manajemen rahasia.
const MAPBOX_ACCESS_TOKEN = "pk.eyJ1Ijoic2lsZWhhZ2UiLCJhIjoiY2wxbnQxejk5MDR2azNrbzEwdDZ2czhhMCJ9.G7SxnM97OX1wz8SAnkMmXg"
const MAPBOX_API_BASE_URL = "https://api.mapbox.com/geocoding/v5/mapbox.places"
const CACHE_DIR = "./cachelatlon" // Direktori untuk menyimpan cache

// Struktur untuk respons Mapbox Geocoding API (disederhanakan)
type MapboxFeature struct {
	PlaceName string    `json:"place_name"`
	Center    []float64 `json:"center"` // [longitude, latitude]
	Text      string    `json:"text"`
	// Anda bisa menambahkan bidang lain yang relevan dari respons Mapbox jika diperlukan
	// Seperti 'relevance', 'properties', dll., jika Anda ingin menampilkannya.
}

type MapboxAPIResponse struct {
	Features []MapboxFeature `json:"features"`
	// Anda bisa menambahkan bidang lain yang relevan dari respons Mapbox jika diperlukan
}


// MapboxGeocodeHandler adalah handler Fiber untuk mengambil data geocoding dari Mapbox
func MapboxGeocodeHandler(c *fiber.Ctx) error {
	// Ambil parameter 'alamat' dari URL
	alamat := c.Query("alamat")
	if alamat == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Parameter 'alamat' diperlukan.",
		})
	}

	// Buat URL Mapbox API
	mapboxURL := fmt.Sprintf("%s/%s.json?access_token=%s", MAPBOX_API_BASE_URL, alamat, MAPBOX_ACCESS_TOKEN)
	log.Printf("Mengambil data dari Mapbox API: %s", mapboxURL)

	// Buat HTTP client dengan timeout
	client := http.Client{
		Timeout: 15 * time.Second,
	}

	// Lakukan permintaan HTTP GET ke Mapbox API
	resp, err := client.Get(mapboxURL)
	if err != nil {
		log.Printf("Kesalahan saat melakukan permintaan ke Mapbox API: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal terhubung ke Mapbox API.",
		})
	}
	defer resp.Body.Close()

	// Baca respons body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Kesalahan saat membaca respons body dari Mapbox API: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal membaca respons dari Mapbox API.",
		})
	}

	// Periksa status kode respons dari Mapbox
	if resp.StatusCode != http.StatusOK {
		log.Printf("Mapbox API mengembalikan status non-OK: %d - %s", resp.StatusCode, string(body))
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"error":   fmt.Sprintf("Mapbox API mengembalikan status %d", resp.StatusCode),
			"details": string(body),
		})
	}

	// Pastikan direktori cache ada
	if err := os.MkdirAll(CACHE_DIR, 0755); err != nil {
		log.Printf("Gagal membuat direktori cache %s: %v", CACHE_DIR, err)
		// Lanjutkan tanpa caching jika gagal membuat direktori
	}

	// Buat nama file unik untuk cache (sederhana: gunakan alamat sebagai nama file)
	cacheFileName := fmt.Sprintf("%s.json", sanitizeFilename(alamat))
	cacheFilePath := filepath.Join(CACHE_DIR, cacheFileName)

	// Simpan respons mentah ke file cache
	if err := ioutil.WriteFile(cacheFilePath, body, 0644); err != nil {
		log.Printf("Gagal menyimpan respons Mapbox ke cache file %s: %v", cacheFilePath, err)
		// Lanjutkan tanpa caching jika gagal menulis file
	} else {
		log.Printf("Respons Mapbox berhasil disimpan ke cache: %s", cacheFilePath)
	}

	// Unmarshal respons untuk diproses
	var mapboxResponse MapboxAPIResponse
	if err := json.Unmarshal(body, &mapboxResponse); err != nil {
		log.Printf("Kesalahan saat mengurai respons JSON Mapbox: %v", err)
		// Jika gagal unmarshal, kirim respons mentah saja
		return c.Status(fiber.StatusOK).Type("application/json").Send(body)
	}

	// --- Perubahan utama ada di bawah sini ---
	// Cek apakah ada fitur yang ditemukan
	if len(mapboxResponse.Features) > 0 {
		// Ambil fitur pertama
		firstFeature := mapboxResponse.Features[0]

		// Kembalikan fitur pertama sebagai respons JSON
		return c.Status(fiber.StatusOK).JSON(firstFeature)
	} else {
		// Jika tidak ada fitur yang ditemukan
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": fmt.Sprintf("Tidak ada data geocoding ditemukan untuk alamat '%s'.", alamat),
		})
	}
}