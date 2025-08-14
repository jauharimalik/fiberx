package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time" // Untuk timeout HTTP client

	"github.com/gorilla/websocket" // Pastikan Anda telah menginstal ini: go get github.com/gorilla/websocket
)

// Struktur untuk mendefinisikan data kendaraan dari API eksternal
type Vehicle struct {
	VehicleId                 int     `json:"VehicleId"`
	VehicleUId                string  `json:"VehicleUId"`
	VehicleName               string  `json:"VehicleName"`
	VehicleRegistrationNumber string  `json:"VehicleRegistrationNumber"`
	GroupName                 string  `json:"GroupName"`
	Latitude                  float64 `json:"Latitude"`
	Longitude                 float64 `json:"Longitude"`
	GpsDate                   string  `json:"GpsDate"`
	Address                   string  `json:"Address"`
	Speed                     float64 `json:"Speed"` // CHANGED: from int to float64
	Course                    float64 `json:"Course"` // CHANGED: from int to float64 based on the JSON example 347.0
	EngineEvent               int     `json:"EngineEvent"`
	EngineEventDate           string  `json:"EngineEventDate"`
	ServerDate                string  `json:"ServerDate"`
	IsPrivate                 bool    `json:"IsPrivate"`
	VehicleIdentificationNumber string `json:"VehicleIdentificationNumber"`
	ExternalPowerVoltage      float64 `json:"ExternalPowerVoltage"`
	ManufactureYear           int     `json:"ManufactureYear"`
}

// Struktur untuk respons API eksternal
type APIResponse struct {
	Payload       []Vehicle `json:"Payload"`
	CorrelationId string    `json:"CorrelationId"`
	IsSuccess     bool      `json:"IsSuccess"`
}

// URL API eksternal
const EXTERNAL_API_URL = "https://albacore-direct-neatly.ngrok-free.app/pkexpress/gettokenarobs"

// Definisikan Upgrader untuk WebSocket
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Izinkan semua origin untuk pengembangan.
		// Untuk produksi, Anda harus membatasi ini ke domain client Anda.
		return true
	},
}

// Fungsi untuk mengambil data dari API eksternal
func fetchVehicleData() (*APIResponse, error) {
	client := http.Client{
		Timeout: 10 * time.Second, // Tambahkan timeout untuk permintaan HTTP
	}
	resp, err := client.Get(EXTERNAL_API_URL)
	if err != nil {
		return nil, fmt.Errorf("gagal melakukan permintaan HTTP ke API eksternal: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API eksternal mengembalikan status non-OK: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca respons body: %w", err)
	}

	var apiResponse APIResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("gagal mengurai respons JSON dari API eksternal: %w", err)
	}

	return &apiResponse, nil
}

// Handler untuk koneksi WebSocket
func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Gagal meng-upgrade koneksi: %v", err)
		return
	}
	defer conn.Close()

	log.Println("Client WebSocket terhubung!")

	for {
		// Membaca pesan dari client (diharapkan VehicleRegistrationNumber)
		// Assign the first return value (messageType) to the blank identifier '_'
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Gagal membaca pesan: %v", err)
			break
		}

		// Pesan yang diterima adalah VehicleRegistrationNumber
		requestedRegNumber := string(message)
		log.Printf("Menerima permintaan untuk VehicleRegistrationNumber: %s", requestedRegNumber)

		// Ambil data dari API eksternal
		apiResponse, err := fetchVehicleData()
		if err != nil {
			log.Printf("Kesalahan saat mengambil data dari API eksternal: %v", err)
			// Kirim pesan error kembali ke client
			conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"error": "Gagal mengambil data kendaraan: %s"}`, err.Error())))
			continue // Lanjutkan mendengarkan pesan lain
		}

		// Cari kendaraan yang cocok
		var foundVehicle *Vehicle
		for i := range apiResponse.Payload {
			if apiResponse.Payload[i].VehicleRegistrationNumber == requestedRegNumber {
				foundVehicle = &apiResponse.Payload[i]
				break
			}
		}

		var responseToClient []byte
		if foundVehicle != nil {
			// Jika ditemukan, marshal objek kendaraan ke JSON
			responseToClient, err = json.MarshalIndent(foundVehicle, "", "  ") // Menggunakan Indent untuk format yang rapi
			if err != nil {
				log.Printf("Gagal mengurai objek kendaraan ke JSON: %v", err)
				conn.WriteMessage(websocket.TextMessage, []byte(`{"error": "Gagal memformat data kendaraan."}`))
				continue
			}
			log.Printf("Mengirim data kendaraan yang ditemukan untuk %s", requestedRegNumber)
		} else {
			// Jika tidak ditemukan
			notFoundMsg := fmt.Sprintf(`{"message": "Data kendaraan dengan nomor registrasi '%s' tidak ditemukan."}`, requestedRegNumber)
			responseToClient = []byte(notFoundMsg)
			log.Printf("Data kendaraan dengan nomor registrasi '%s' tidak ditemukan.", requestedRegNumber)
		}

		// Kirim respons kembali ke client
		if err := conn.WriteMessage(websocket.TextMessage, responseToClient); err != nil {
			log.Printf("Gagal mengirim balasan ke client: %v", err)
			break
		}
	}
}

func Cekplat() {
	// Menentukan endpoint WebSocket
	http.HandleFunc("/ws", wsHandler)

	// Mulai server HTTP di port 4545
	log.Println("Server WebSocket berjalan di ws://192.168.60.19:4245:4545/ws")
	err := http.ListenAndServe(":4545", nil)
	if err != nil {
		log.Fatalf("Gagal memulai server: %v", err)
	}
}