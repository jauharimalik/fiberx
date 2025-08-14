package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	_ "github.com/denisenkom/go-mssqldb" // SQL Server driver
	"my-fiber-app/db"                   // Import 'db' package for database connection
)

// Mapbox API configuration
const (
	mapboxDirectionsURL   = "https://api.mapbox.com/directions/v5/mapbox/driving"
	originLonx             = 106.8712457                                                                                   // Fixed Origin Longitude (example: your depo)
	originLatx             = -6.1365156                                                                                    // Fixed Origin Latitude (example: your depo)
)

// MapboxDirectionsResponse represents the relevant parts of the Mapbox Directions API response.
type MapboxDirectionsResponse struct {
	Routes []struct {
		Distance float64 `json:"distance"` // Distance in meters
	} `json:"routes"`
	Code string `json:"code"` // "Ok" if successful
}

// getCustomersWithNullJarak retrieves up to 100 customers whose 'jarak' (distance) is NULL,
// but 'lat' and 'lon' are not NULL.
func getCustomersWithNullJarak(dbConn *sql.DB) ([]Customer, error) {
	query := `
		SELECT DISTINCT TOP 100 t0.id, t0.cardcode, t0.cardname, t0.address as mailaddres,
		t0.alamat as address, t0.zipcode, t0.lat, t0.lon, t0.jarak
		FROM [pksrv-sap].pk_express.dbo.master_customer t0
		INNER JOIN [pk-query].db_santosh.dbo.b_cust t1 ON t0.cardcode = t1.cardcode
		WHERE t0.jarak IS NULL
		AND t0.alamat != '' AND t0.alamat != ',' AND t0.alamat NOT LIKE '%null%' AND t0.alamat != ', JAKARTA'
		AND t0.alamat != 'PANDURASA, N'
		AND t1.city != 'Null'
		AND (t1.city LIKE '%jakarta%' OR t1.city LIKE '%bogor%' OR t1.city LIKE '%bekasi%' OR t1.city LIKE '%depok%' OR t1.city LIKE '%tangerang%');
	`

	rows, err := dbConn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying customers with null jarak: %w", err)
	}
	defer rows.Close()

	var customers []Customer
	for rows.Next() {
		var c Customer
		err := rows.Scan(
			&c.ID,
			&c.CardCode,
			&c.CardName,
			&c.Address,
			&c.MailAddress,
			&c.ZipCode,
			&c.Lat,
			&c.Lon,
			&c.Jarak,
		)
		if err != nil {
			log.Printf("Error scanning customer row for jarak update: %v", err)
			continue
		}
		customers = append(customers, c)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating customer rows for jarak update: %w", err)
	}

	return customers, nil
}

// getDistance calculates the driving distance between two coordinates using Mapbox Directions API.
// Coordinates are in [longitude, latitude] format.
func getDistance(destLon, destLat float64) (float64, error) {
	// Mapbox Directions API expects coordinates in lon,lat;lon,lat format
	coordinates := fmt.Sprintf("%.6f,%.6f;%.6f,%.6f", originLonx, originLatx, destLon, destLat)
	directionsURL := fmt.Sprintf("%s/%s?access_token=%s", mapboxDirectionsURL, coordinates, mapboxAccessToken)

	log.Printf("Fetching distance for coordinates: %s", coordinates)

	client := http.Client{
		Timeout: 5 * time.Second, // Increased timeout for Directions API
	}

	resp, err := client.Get(directionsURL)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to Mapbox Directions API: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read Mapbox Directions API response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Mapbox Directions API returned non-OK status: %d - %s", resp.StatusCode, string(body))
	}

	var directionsResponse MapboxDirectionsResponse
	if err := json.Unmarshal(body, &directionsResponse); err != nil {
		return 0, fmt.Errorf("failed to parse Mapbox Directions JSON response: %w", err)
	}

	if directionsResponse.Code != "Ok" || len(directionsResponse.Routes) == 0 {
		return 0, fmt.Errorf("no route found or Mapbox Directions API error: %s", directionsResponse.Code)
	}

	// The distance is returned in meters; convert to kilometers if desired (optional)
	distanceInMeters := directionsResponse.Routes[0].Distance
	log.Printf("Found distance for %s: %.2f meters", coordinates, distanceInMeters)
	return distanceInMeters, nil // Return distance in meters
}

// updateCustomerJarak updates customer's 'jarak' (distance) in the database.
func updateCustomerJarak(dbConn *sql.DB, customerID int, jarak float64) error {
	query := `UPDATE [pksrv-sap].pk_express.dbo.master_customer SET jarak = @p1 WHERE id = @p2;`
	_, err := dbConn.Exec(query, jarak, customerID)
	if err != nil {
		return fmt.Errorf("error updating jarak for customer ID %d: %w", customerID, err)
	}
	log.Printf("Successfully updated Jarak for customer ID: %d to %.2f", customerID, jarak)
	return nil
}

// StartJarakUpdater starts the periodic customer distance update process.
// This function will run as a background goroutine.
func StartJarakUpdater() {
	if db.DB == nil {
		log.Fatal("Database connection not initialized. Call db.Connect() before StartJarakUpdater.")
	}

	// Use a longer ticker for distance calculation as it might be less frequent.
	// For example, every 30 seconds or 1 minute.
	ticker := time.NewTicker(10 * time.Second) // Runs every 30 seconds
	defer ticker.Stop()

	for range ticker.C {
		log.Println("Starting Jarak (Distance) update cycle...")
		customers, err := getCustomersWithNullJarak(db.DB)
		if err != nil {
			log.Printf("Error getting customers with null jarak: %v", err)
			continue
		}

		if len(customers) == 0 {
			log.Println("No customers found with null Jarak. Skipping this cycle.")
			continue
		}

		for _, customer := range customers {
			// Ensure Lat and Lon are valid before proceeding
			if !customer.Lat.Valid || !customer.Lon.Valid {
				log.Printf("Skipping customer ID %d: Lat or Lon is NULL, cannot calculate distance.", customer.ID)
				continue
			}

			distance, err := getDistance(customer.Lon.Float64, customer.Lat.Float64)
			if err != nil {
				log.Printf("Error calculating distance for customer ID %d (Lat: %.6f, Lon: %.6f): %v",
					customer.ID, customer.Lat.Float64, customer.Lon.Float64, err)
				continue
			}

			err = updateCustomerJarak(db.DB, customer.ID, distance)
			if err != nil {
				log.Printf("Error updating jarak for customer ID %d: %v", customer.ID, err)
				continue
			}
		}
		log.Println("Finished Jarak (Distance) update cycle.")
	}
}

/*
// Example of how to call this in your main function or a router setup:
func main() {
	// Initialize database connection first
	// For example:
	// db.Connect("sqlserver://user:password@server/database")

	// Start the Lat/Lon updater in a goroutine
	go StartLatLonUpdater()

	// Start the Jarak (Distance) updater in a goroutine
	go StartJarakUpdater()

	// Keep the main goroutine running (e.g., start your Fiber app)
	// app := fiber.New()
	// app.Listen(":3000")
}
*/