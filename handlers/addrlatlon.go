package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"net/url"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
)


// Coordinates struct untuk menyimpan hasil latitude dan longitude
type Coordinates struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
}

const geocodeCacheFile = "geocode_cache.json"

// readGeocodeCache membaca data geocode dari file cache.
func readGeocodeCache(address string) (*Coordinates, error) {
	cacheDir := "./cache"
	cachePath := filepath.Join(cacheDir, fmt.Sprintf("geocode_%s.json", url.QueryEscape(address)))
	data, err := ioutil.ReadFile(cachePath)
	if os.IsNotExist(err) {
		fmt.Printf("Cache file for '%s' not found.\n", address)
		return nil, nil // Cache tidak ada, bukan error
	}
	if err != nil {
		fmt.Printf("Error reading cache file for '%s': %v\n", address, err)
		return nil, err
	}
	fmt.Printf("Cache file for '%s' read successfully.\n", address)
	var coords Coordinates
	err = json.Unmarshal(data, &coords)
	return &coords, err
}

// writeGeocodeCache menulis data geocode ke file cache.
func writeGeocodeCache(address string, coords *Coordinates) error {
	cacheDir := "./cache"
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		fmt.Println("Creating cache directory.")
		os.MkdirAll(cacheDir, 0755)
	}
	cachePath := filepath.Join(cacheDir, fmt.Sprintf("geocode_%s.json", url.QueryEscape(address)))
	data, err := json.Marshal(coords)
	if err != nil {
		fmt.Println("Error marshaling coordinates to JSON:", err)
		return err
	}
	err = ioutil.WriteFile(cachePath, data, 0644)
	if err != nil {
		fmt.Printf("Error writing to cache for '%s': %v\n", address, err)
	} else {
		fmt.Printf("Data for '%s' written to cache.\n", address)
	}
	return err
}


// NewAddrlatlon handles the request to get latitude and longitude from an address via GET parameter.
func NewAddrlatlon(c *fiber.Ctx) error {
	address := c.Params("addr")
	if address == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Parameter 'addr' tidak boleh kosong")
	}

	apiKey := "abcf3b4798af4be4b4747346c33db394" // Ganti dengan API key Geoapify Anda

	// Coba ambil dari cache terlebih dahulu
	cachedCoords, err := readGeocodeCache(address)
	if err != nil {
		fmt.Println("Error reading cache:", err)
	}
	if cachedCoords != nil {
		fmt.Println("Serving from cache.")
		c.JSON(cachedCoords)
		return nil
	}

	// Jika tidak ada di cache, panggil API Geoapify
	baseURL := "https://api.geoapify.com/v1/geocode/search"
	u, err := url.Parse(baseURL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Error parsing URL: %v", err))
	}

	queryParams := u.Query()
	queryParams.Set("text", address)
	queryParams.Set("apiKey", apiKey)
	u.RawQuery = queryParams.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Error calling Geoapify API: %v", err))
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Error reading Geoapify response: %v", err))
	}

	var responseData map[string]interface{}
	err = json.Unmarshal(body, &responseData)
	if err != nil {
		fmt.Println("Error decoding Geoapify JSON:", err)
		fmt.Println("Raw Response:", string(body))
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Error decoding Geoapify response: %v", err))
	}

	features, ok := responseData["features"].([]interface{})
	if !ok || len(features) == 0 {
		return c.Status(fiber.StatusNotFound).SendString(fmt.Sprintf("Koordinat untuk alamat '%s' tidak ditemukan", address))
	}

	firstFeature, ok := features[0].(map[string]interface{})
	if !ok {
		return c.Status(fiber.StatusInternalServerError).SendString("Error processing Geoapify response (features)")
	}

	geometry, ok := firstFeature["geometry"].(map[string]interface{})
	if !ok {
		return c.Status(fiber.StatusInternalServerError).SendString("Error processing Geoapify response (geometry)")
	}

	coordinates, ok := geometry["coordinates"].([]interface{})
	if !ok || len(coordinates) != 2 {
		return c.Status(fiber.StatusInternalServerError).SendString("Error processing Geoapify response (coordinates)")
	}

	longitude, ok := coordinates[0].(float64)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).SendString("Error processing Geoapify response (longitude)")
	}

	latitude, ok := coordinates[1].(float64)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).SendString("Error processing Geoapify response (latitude)")
	}

	coords := &Coordinates{
		Latitude:  latitude,
		Longitude: longitude,
	}

	// Simpan ke cache
	err = writeGeocodeCache(address, coords)
	if err != nil {
		fmt.Println("Error writing to cache:", err)
	}

	c.JSON(coords)
	return nil
}