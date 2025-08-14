package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http" // Tetap digunakan untuk panggilan API eksternal Mapbox
	"net/url"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2" // Import Fiber di sini
)

const (
	mapboxGeocodingAPI = "https://api.mapbox.com/geocoding/v5/mapbox.places/%s.json"
	mapboxMatrixAPI    = "https://api.mapbox.com/distances/v1/mapbox/driving/%s?sources=%s&destinations=%s&access_token=%s"
	originLat          = -6.1321169
	originLon          = 106.8368375
	kmPerLiter         = 6.0 // Konsumsi BBM: 6 km per liter
)

// GeocodingResponse represents the structure of the Mapbox Geocoding API response
type GeocodingResponse struct {
	Features []struct {
		PlaceName  string    `json:"place_name"`
		Center     []float64 `json:"center"` // [longitude, latitude]
		Properties struct {
			Address  string `json:"address"`
			Postcode string `json:"postcode"`
		} `json:"properties"`
	} `json:"features"`
}

// DistanceMatrixResponse represents the structure of the Mapbox Distance Matrix API response
type DistanceMatrixResponse struct {
	Distances [][]float64 `json:"distances"`
	Durations [][]float64 `json:"durations"`
}

// JarakInfo represents the information to be returned to the user
type JarakInfo struct {
	Alamat           string  `json:"alamat"`
	Kodepos          string  `json:"kodepos"`
	JarakKM          float64 `json:"jarak_km"`
	Lat              float64 `json:"latitude"`
	Lon              float64 `json:"longitude"`
	PerkiraanWaktu   string  `json:"perkiraan_waktu_tempuh"`
	KonsumsiBBMLiter float64 `json:"konsumsi_bbm_liter"`
}

// mapboxAccessToken sekarang adalah konstanta dan tidak lagi membaca dari environment variable
const mapboxAccessToken = "pk.eyJ1Ijoic2lsZWhhZ2UiLCJhIjoiY2wxbnQxejk5MDR2azNrbzEwdDZ2czhhMCJ9.G7SxnM97OX1wz8SAnkMmXg"

// Hapus fungsi init() jika Anda tidak lagi memerlukan pembacaan environment variable untuk token
// func init() {
// 	// Ini tidak lagi diperlukan karena mapboxAccessToken sekarang adalah konstanta.
// 	// Jika Anda punya inisialisasi lain di sini, biarkan bagian itu.
// }

// KonversijarakHandler sekarang menggunakan *fiber.Ctx
func KonversijarakHandler(c *fiber.Ctx) error {
	alamatQuery := c.Query("alamat")
	if alamatQuery == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Parameter 'alamat' diperlukan.",
		})
	}

	// 1. Geocoding: Convert address to coordinates
	targetLat, targetLon, placeName, postcode, err := geocodeAddress(alamatQuery)
	if err != nil {
		log.Printf("Error geocoding address '%s': %v", alamatQuery, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Gagal mengonversi alamat ke koordinat.",
			"details": err.Error(),
		})
	}

	// --- PENTING: Penanganan jika geocoding tidak menemukan hasil yang valid ---
	if placeName == "" || (targetLat == 0.0 && targetLon == 0.0) {
		log.Printf("Geocoding returned no valid results for address: %s. Lat: %f, Lon: %f, Place: %s", alamatQuery, targetLat, targetLon, placeName)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Alamat tujuan tidak ditemukan atau tidak dapat di-geocode menjadi koordinat yang valid.",
			"query": alamatQuery,
		})
	}
	log.Printf("Successfully geocoded '%s' to: %s (Lat: %f, Lon: %f, Postcode: %s)", alamatQuery, placeName, targetLat, targetLon, postcode)
	// --- AKHIR PENANGANAN ---


	// 2. Distance Matrix: Calculate distance and duration
	distanceMeters, durationSeconds, err := getDistanceAndDuration(originLat, originLon, targetLat, targetLon)
	if err != nil {
		log.Printf("Error getting distance matrix for '%s': %v", alamatQuery, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Gagal mendapatkan informasi jarak dan waktu tempuh.",
			"details": err.Error(),
		})
	}

	distanceKM := distanceMeters / 1000.0
	konsumsiBBM := distanceKM / kmPerLiter

	// Format duration
	duration := time.Duration(durationSeconds) * time.Second
	perkiraanWaktu := formatDuration(duration)

	info := JarakInfo{
		Alamat:           placeName,
		Kodepos:          postcode,
		JarakKM:          distanceKM,
		Lat:              targetLat,
		Lon:              targetLon,
		PerkiraanWaktu:   perkiraanWaktu,
		KonsumsiBBMLiter: konsumsiBBM,
	}

	return c.JSON(info)
}

// geocodeAddress converts an address string to latitude, longitude, place name, and postcode
func geocodeAddress(address string) (lat, lon float64, placeName, postcode string, err error) {
	encodedAddress := url.QueryEscape(address)
	geocodingURL := fmt.Sprintf(mapboxGeocodingAPI, encodedAddress) + "?access_token=" + mapboxAccessToken

	log.Printf("Geocoding API Request URL: %s", geocodingURL)

	resp, err := http.Get(geocodingURL)
	if err != nil {
		return 0, 0, "", "", fmt.Errorf("failed to make HTTP request to geocoding API: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, "", "", fmt.Errorf("failed to read geocoding API response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Geocoding API returned non-OK status %d: %s", resp.StatusCode, string(body))
		return 0, 0, "", "", fmt.Errorf("geocoding API returned status %d: %s", resp.StatusCode, string(body))
	}

	var geoResp GeocodingResponse
	if err := json.Unmarshal(body, &geoResp); err != nil {
		return 0, 0, "", "", fmt.Errorf("failed to unmarshal geocoding API response: %w", err)
	}

	if len(geoResp.Features) == 0 {
		return 0, 0, "", "", fmt.Errorf("no results found for address: %s", address)
	}

	feature := geoResp.Features[0]
	lat = feature.Center[1] // Mapbox returns [lon, lat]
	lon = feature.Center[0]
	placeName = feature.PlaceName
	postcode = feature.Properties.Postcode

	return lat, lon, placeName, postcode, nil
}

// getDistanceAndDuration calculates distance and duration between two points using Mapbox Distance Matrix API
func getDistanceAndDuration(lat1, lon1, lat2, lon2 float64) (distanceMeters, durationSeconds float64, err error) {
	coordinates := fmt.Sprintf("%f,%f;%f,%f", lon1, lat1, lon2, lat2) // Mapbox expects lon,lat
	sources := "0"    // Index of the origin coordinate
	destinations := "1" // Index of the destination coordinate

	matrixURL := fmt.Sprintf(mapboxMatrixAPI, coordinates, sources, destinations, mapboxAccessToken)

	log.Printf("Distance Matrix API Request URL: %s", matrixURL)

	resp, err := http.Get(matrixURL)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to make HTTP request to matrix API: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read matrix API response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Distance Matrix API returned non-OK status %d: %s", resp.StatusCode, string(body))
		return 0, 0, fmt.Errorf("matrix API returned status %d: %s", resp.StatusCode, string(body))
	}

	var matrixResp DistanceMatrixResponse
	if err := json.Unmarshal(body, &matrixResp); err != nil {
		return 0, 0, fmt.Errorf("failed to unmarshal matrix API response: %w", err)
	}

	if len(matrixResp.Distances) == 0 || len(matrixResp.Distances[0]) == 0 {
		return 0, 0, fmt.Errorf("no distances found in matrix API response")
	}
	if len(matrixResp.Durations) == 0 || len(matrixResp.Durations[0]) == 0 {
		return 0, 0, fmt.Errorf("no durations found in matrix API response")
	}

	distanceMeters = matrixResp.Distances[0][0]
	durationSeconds = matrixResp.Durations[0][0]

	return distanceMeters, durationSeconds, nil
}

// formatDuration formats time.Duration into a human-readable string
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	parts := []string{}
	if days > 0 {
		parts = append(parts, strconv.Itoa(days)+" hari")
	}
	if hours > 0 {
		parts = append(parts, strconv.Itoa(hours)+" jam")
	}
	if minutes > 0 {
		parts = append(parts, strconv.Itoa(minutes)+" menit")
	}
	if seconds > 0 && len(parts) == 0 { // Only show seconds if no larger units are present
		parts = append(parts, strconv.Itoa(seconds)+" detik")
	}
	if len(parts) == 0 {
		return "0 detik"
	}
	return joinStrings(parts, " ")
}

func joinStrings(s []string, sep string) string {
	if len(s) == 0 {
		return ""
	}
	if len(s) == 1 {
		return s[0]
	}
	result := s[0]
	for i := 1; i < len(s); i++ {
		result += sep + s[i]
	}
	return result
}