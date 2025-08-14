package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url" // Import for URL escaping
	"strings"   // Import for string manipulation
	"time"

	_ "github.com/denisenkom/go-mssqldb" // SQL Server driver for database connection
	"my-fiber-app/db"                    // Import 'db' package for database connection
)

// Mapbox API configuration
const (
	mapboxAPIBaseURL  = "https://api.mapbox.com/geocoding/v5/mapbox.places"
)

// Customer represents the customer data structure from the master_customer table.
// Uses sql.Null* types for columns that can be NULL in the database.
type Customer struct {
	ID          int
	CardCode    sql.NullString
	CardName    sql.NullString
	Address     sql.NullString
	MailAddress sql.NullString
	ZipCode     sql.NullString
	Lat         sql.NullFloat64 // Latitude, can be NULL
	Lon         sql.NullFloat64 // Longitude, can be NULL
	Jarak       sql.NullFloat64 // Distance, can be NULL
}

// MapboxFeaturex represents a geocoding feature from the Mapbox response.
type MapboxFeaturex struct {
	PlaceName string    `json:"place_name"` // Name of the place found
	Center    []float64 `json:"center"`     // Coordinates [longitude, latitude]
	Text      string    `json:"text"`       // Short text description of the place
}

// MapboxAPIResponsex represents the complete response from the Mapbox Geocoding API.
type MapboxAPIResponsex struct {
	Features []MapboxFeaturex `json:"features"` // List of geocoding features
}

// getCustomersToUpdate retrieves up to 100 customers whose lat, lon, or jarak are still NULL from the database.
func getCustomersToUpdate(dbConn *sql.DB) ([]Customer, error) {
	// SQL query to select customers that need coordinate updates.
	query := `SELECT DISTINCT TOP 100 t0.id, t0.cardcode, t0.cardname, t0.address as mailaddres,
			 t0.alamat as address, t0.zipcode, t0.lat, t0.lon, t0.jarak
			 FROM [pksrv-sap].pk_express.dbo.master_customer t0
			 INNER JOIN [pk-query].db_santosh.dbo.b_cust t1 ON t0.cardcode = t1.cardcode
			 WHERE  (t0.lat IS NULL OR t0.lon IS NULL) and
			 t0.alamat != '' and t0.alamat != ',' and t0.alamat not like '%null%'  and t0.alamat != ', JAKARTA'
			 and t0.alamat != 'PANDURASA, N'
			 AND t1.city != 'Null'
			 AND (t1.city LIKE '%jakarta%' OR t1.city LIKE '%bogor%' OR t1.city LIKE '%bekasi%' OR t1.city LIKE '%depok%' OR t1.city LIKE '%tangerang%');`

	rows, err := dbConn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying customers: %w", err)
	}
	defer rows.Close() // Ensure rows are closed after finishing

	var customers []Customer
	for rows.Next() {
		var c Customer
		// Scan row results into the Customer struct.
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
			log.Printf("Error scanning customer row: %v", err)
			continue // Continue to the next row if there's a scan error
		}
		customers = append(customers, c) // Add customer to the slice
	}

	// Check for any errors that may have occurred during row iteration.
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating customer rows: %w", err)
	}

	return customers, nil
}

// cleanAddress prepares the address for Mapbox API by removing problematic characters and potentially truncating.
func cleanAddress(address string) string {
	// Remove carriage returns and hash symbols
	cleaned := strings.ReplaceAll(address, "\r", "")
	cleaned = strings.ReplaceAll(cleaned, "#", "")

	// Remove multiple spaces
	cleaned = strings.Join(strings.Fields(cleaned), " ")

	// Optional: More aggressive cleaning or truncation based on common patterns
	// Example: remove "RT " and "RW " if they are often followed by numbers and causing issues
	// cleaned = regexp.MustCompile(`RT\s+\d+`).ReplaceAllString(cleaned, "")
	// cleaned = regexp.MustCompile(`RW\s+\d+`).ReplaceAllString(cleaned, "")

	// Truncate to a reasonable length if still too long.
	// Mapbox token limit is around 20 tokens, which is roughly 100-150 characters for English.
	// You might need to adjust this limit based on your address data.
	if len(cleaned) > 100 { // Example: truncate to 100 characters
		cleaned = cleaned[:100]
	}

	return cleaned
}

// getCoordinatesFromMapbox retrieves coordinates (lat, lon) from Mapbox based on the address.
func getCoordinatesFromMapbox(address string) (float64, float64, error) {
	if address == "" {
		return 0, 0, fmt.Errorf("address cannot be empty for Mapbox query")
	}

	// Clean and URL-escape the address before sending to Mapbox
	cleanedAddress := cleanAddress(address)
	escapedAddress := url.QueryEscape(cleanedAddress) // URL-escape the cleaned address

	// Form the Mapbox API URL.
	mapboxURL := fmt.Sprintf("%s/%s.json?access_token=%s", mapboxAPIBaseURL, escapedAddress, mapboxAccessToken)
	log.Printf("Fetching coordinates for address: %s (cleaned: %s)", address, cleanedAddress)

	// Create an HTTP client with a timeout.
	client := http.Client{
		Timeout: 3 * time.Second, // Timeout remains 3 seconds
	}

	resp, err := client.Get(mapboxURL)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to connect to Mapbox API: %w", err)
	}
	defer resp.Body.Close() // Ensure the response body is closed

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read Mapbox API response body: %w", err)
	}

	// Check the response status code from Mapbox.
	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("Mapbox API returned non-OK status: %d - %s", resp.StatusCode, string(body))
	}

	var mapboxResponse MapboxAPIResponsex // Using MapboxAPIResponsex
	// Unmarshal the Mapbox JSON response into the struct.
	if err := json.Unmarshal(body, &mapboxResponse); err != nil {
		return 0, 0, fmt.Errorf("failed to parse Mapbox JSON response: %w", err)
	}

	// If features are found, get the first coordinate.
	if len(mapboxResponse.Features) > 0 {
		// Mapbox returns coordinates in [longitude, latitude] format.
		lon := mapboxResponse.Features[0].Center[0]
		lat := mapboxResponse.Features[0].Center[1]
		log.Printf("Found coordinates for '%s': Lat=%.6f, Lon=%.6f", cleanedAddress, lat, lon)
		return lat, lon, nil
	}

	return 0, 0, fmt.Errorf("no geocoding data found for address '%s' (original: '%s')", cleanedAddress, address)
}

// updateCustomerCoordinates updates customer's lat and lon in the database.
func updateCustomerCoordinates(dbConn *sql.DB, customerID int, lat, lon float64) error {
	// Query still uses '@p1', '@p2', '@p3' for SQL Server compatibility.
	query := `UPDATE [pksrv-sap].pk_express.dbo.master_customer SET lat = @p1, lon = @p2 WHERE id = @p3;`
	_, err := dbConn.Exec(query, lat, lon, customerID) // Execute the update query
	if err != nil {
		return fmt.Errorf("error updating customer ID %d: %w", customerID, err)
	}
	log.Printf("Successfully updated Lat/Lon for customer ID: %d", customerID)
	return nil
}

// StartLatLonUpdater starts the periodic customer coordinate update process.
// This function will run as a background goroutine.
func StartLatLonUpdater() {
	// Ensure the database connection has been initialized by the 'db' package.
	if db.DB == nil {
		log.Fatal("Database connection not initialized. Call db.Connect() before StartLatLonUpdater.")
	}

	// Create a ticker that will send a signal every 10 seconds.
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop() // Ensure the ticker is stopped when the function exits

	// Main loop that will run every time the ticker sends a signal.
	for range ticker.C {

		// Retrieve customers that need to be updated.
		customers, err := getCustomersToUpdate(db.DB) // Using the global database connection from the 'db' package
		if err != nil {
			log.Printf("Error getting customers to update: %v", err)
			continue // Continue to the next cycle if there's an error
		}

		// Iterate through each customer found.
		for _, customer := range customers {
			// Prioritize MailAddress for geocoding; if empty, use Address.
			addressToGeocode := customer.MailAddress.String
			if !customer.MailAddress.Valid || addressToGeocode == "" {
				addressToGeocode = customer.Address.String
				if !customer.Address.Valid || addressToGeocode == "" {
					log.Printf("Skipping customer ID %d: Both MailAddress and Address are empty or invalid.", customer.ID)
					continue // Skip customer if both addresses are empty
				}
			}

			// Get coordinates from Mapbox.
			lat, lon, err := getCoordinatesFromMapbox(addressToGeocode)
			if err != nil {
				log.Printf("Error getting coordinates for customer ID %d (Address: %s): %v", customer.ID, addressToGeocode, err)
				continue // Skip customer if geocoding fails
			}

			// Update coordinates in the database.
			err = updateCustomerCoordinates(db.DB, customer.ID, lat, lon) // Using the global database connection
			if err != nil {
				log.Printf("Error updating coordinates for customer ID %d: %v", customer.ID, err)
				continue // Skip customer if database update fails
			}
		}
	}
}