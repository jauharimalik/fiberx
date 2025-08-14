package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// AuthResponse struct untuk mendekode respons token
type AuthResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

const vehiclesCacheFile = "vehicles_cache.json"

// readVehiclesCache membaca data kendaraan dari file cache.
func readVehiclesCache() ([]byte, error) {
	cachePath := filepath.Join("./cache", vehiclesCacheFile)
	data, err := ioutil.ReadFile(cachePath)
	if os.IsNotExist(err) {
		fmt.Println("Cache file not found.")
		return nil, nil // Cache tidak ada, bukan error
	}
	if err != nil {
		fmt.Println("Error reading cache file:", err)
		return nil, err
	}
	fmt.Println("Cache file read successfully.")
	return data, nil
}

// writeVehiclesCache menulis data kendaraan ke file cache.
func writeVehiclesCache(data []byte) error {
	cacheDir := "./cache"
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		fmt.Println("Creating cache directory.")
		os.MkdirAll(cacheDir, 0755)
	}
	cachePath := filepath.Join(cacheDir, vehiclesCacheFile)
	err := ioutil.WriteFile(cachePath, data, 0644)
	if err != nil {
		fmt.Println("Error writing to cache:", err)
	} else {
		fmt.Println("Data written to cache.")
	}
	return err
}

// isValidJSON checks if the given byte slice is a valid JSON.
func isValidJSON(b []byte) bool {
	var js json.RawMessage
	return json.Unmarshal(b, &js) == nil
}

// Gettokenarobs handles the request to retrieve the token and then fetches company vehicles with cache.
func Gettokenarobs(c *fiber.Ctx) error {
	// Langkah 1: Mendapatkan Token
	tokenURL := "https://api.trackgps.ro/api/authentication/login?api-version=2.0"
	payload := strings.NewReader("-----011000010111000001101001\r\nContent-Disposition: form-data; name=\"username\"\r\n\r\njauhari@pandurasa.com\r\n-----011000010111000001101001\r\nContent-Disposition: form-data; name=\"password\"\r\n\r\nPandurasa12345\r\n-----011000010111000001101001--\r\n")

	req, err := http.NewRequest("POST", tokenURL, payload)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Error creating token request: %v", err))
	}
	req.Header.Add("Content-Type", "multipart/form-data; boundary=---011000010111000001101001")

	tokenRes, err := http.DefaultClient.Do(req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Error sending token request: %v", err))
	}
	defer tokenRes.Body.Close()

	tokenBodyBytes, err := ioutil.ReadAll(tokenRes.Body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Error reading token response: %v", err))
	}

	var authResponse AuthResponse
	err = json.Unmarshal(tokenBodyBytes, &authResponse)
	if err != nil {
		fmt.Println("Error decoding JSON:", err)
		fmt.Println("Raw Response:", string(tokenBodyBytes))
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Error decoding token response: %v", err))
	}

	accessToken := authResponse.AccessToken
	fmt.Println("Access Token:", accessToken)

	// Langkah 2: Menggunakan Token sebagai Bearer untuk Permintaan Lain
	vehiclesURL := "https://api.trackgps.ro/api/carriers/company-vehicles?api-version=2.0"
	vehiclesReq, err := http.NewRequest("GET", vehiclesURL, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Error creating vehicles request: %v", err))
	}

	vehiclesReq.Header.Add("Authorization", "Bearer "+accessToken)

	vehiclesRes, err := http.DefaultClient.Do(vehiclesReq)
	if err != nil {
		fmt.Println("Error sending vehicles request:", err)
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Error sending vehicles request: %v", err))
	}
	defer vehiclesRes.Body.Close()

	vehiclesBodyBytes, err := ioutil.ReadAll(vehiclesRes.Body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Error reading vehicles response: %v", err))
	}

	fmt.Println("Vehicles Response Status:", vehiclesRes.StatusCode)
	fmt.Println("Vehicles Response Body:", string(vehiclesBodyBytes))

	// Periksa status kode untuk rate limit atau jika respons bukan JSON
	if vehiclesRes.StatusCode == http.StatusTooManyRequests || !isValidJSON(vehiclesBodyBytes) {
		fmt.Println("API calls quota exceeded or invalid JSON. Attempting to retrieve from cache.")
		cachedData, cacheErr := readVehiclesCache()
		if cacheErr != nil {
			fmt.Println("Error reading from cache:", cacheErr)
			return c.Status(fiber.StatusInternalServerError).SendString("API quota exceeded/invalid JSON and error reading cache")
		}
		if cachedData != nil {
			fmt.Println("Serving data from cache.")
			c.Set("Content-Type", "application/json")
			return c.Send(cachedData)
		} else {
			return c.Status(http.StatusTooManyRequests).SendString("API quota exceeded/invalid JSON and no cache available")
		}
	}

	// Simpan hasil ke cache jika permintaan berhasil dan respons adalah JSON
	if vehiclesRes.StatusCode >= 200 && vehiclesRes.StatusCode < 300 && isValidJSON(vehiclesBodyBytes) {
		err = writeVehiclesCache(vehiclesBodyBytes)
		if err != nil {
			fmt.Println("Error writing to cache:", err)
		}
	}

	c.Set("Content-Type", "application/json")
	return c.Send(vehiclesBodyBytes) // Kirimkan respons dari endpoint vehicles
}