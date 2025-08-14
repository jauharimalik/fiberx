package handlers

import (
	"database/sql"
	"encoding/json"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/gofiber/fiber/v2"
	"my-fiber-app/db"
)

// Define the default cache duration. Data will be considered fresh for this period.
const defaultCacheDuration = 1 / 2 * time.Minute

// CacheBehavior constants
const (
	CacheBehaviorAll    = "all"
	CacheBehaviorRead   = "read"
	CacheBehaviorCreate = "create"
)

// GenericHtmlQueryHandler executes a SQL query and returns the results
// formatted as an HTML table. It now implements a file-based caching mechanism
// with dynamic expiration and behavior.
// It accepts an optional 'columnOrder' parameter to control the order of columns in the HTML table.
// If columnOrder is nil or empty, it will derive the order from the query results.
// The last two optional parameters are cacheDurationMinutes (int) and cacheBehavior (string).
func GenericHtmlQueryHandler(c *fiber.Ctx, dbName string, query string, columnOrder []string, opts ...interface{}) error {
	// Parse optional parameters: cacheDurationMinutes and cacheBehavior
	var cacheDurationToUse time.Duration = defaultCacheDuration
	var cacheBehaviorToUse string = CacheBehaviorAll
	var directQueryParams []interface{} // Parameters passed directly to the handler call

	// Extract query parameters from Fiber context (GET, POST/JSON body, Form data)
	// These are used for generating the cache file name to make it unique per request parameters.
	httpContextParams := extractQueryParamsFromContext(c)

	// Iterate through opts to find cacheDurationMinutes and cacheBehavior,
	// and collect any remaining explicit query parameters.
	for _, opt := range opts {
		switch v := opt.(type) {
		case int:
			if v == 0 { // A duration of 0 means no caching, so set duration to 0 and behavior to 'create'
				cacheDurationToUse = 0
				cacheBehaviorToUse = CacheBehaviorCreate // Forces fetch, but no write/read if duration is 0
			} else if v > 0 {
				cacheDurationToUse = time.Duration(v) * time.Minute
			}
		case string:
			switch strings.ToLower(v) {
			case CacheBehaviorRead:
				cacheBehaviorToUse = CacheBehaviorRead
			case CacheBehaviorCreate:
				cacheBehaviorToUse = CacheBehaviorCreate
			case CacheBehaviorAll:
				cacheBehaviorToUse = CacheBehaviorAll
			default:
				directQueryParams = append(directQueryParams, opt)
			}
		default:
			directQueryParams = append(directQueryParams, opt)
		}
	}

	var finalQueryParams []interface{}
	finalQueryParams = append(finalQueryParams, directQueryParams...)
	for _, p := range httpContextParams {
		finalQueryParams = append(finalQueryParams, p)
	}

	if query == "" {
		return c.Status(400).SendString("Query parameter cannot be empty")
	}

	selectedDBName := dbName
	if selectedDBName == "" {
		selectedDBName = db.DefaultDBName
	}

	dbConn := db.DB
	if dbConn == nil {
		log.Println("Database connection is nil. Ensure db.Connect() was called.")
		return c.Status(500).SendString("Database connection is nil. Ensure db.Connect() was called.")
	}

	cacheBaseName := fmt.Sprintf("html_%s", sanitizeFilename(query)) 
	if len(finalQueryParams) > 0 {
		paramStrings := make([]string, len(finalQueryParams))
		for i, p := range finalQueryParams {
			paramStrings[i] = fmt.Sprintf("%v", p)
		}
		cacheBaseName += "_" + sanitizeFilename(strings.Join(paramStrings, "_"))
	}
	cacheFileName := fmt.Sprintf("%s.html", cacheBaseName)
	cacheFilePath := filepath.Join("cache", cacheFileName)

	if _, err := os.Stat("cache"); os.IsNotExist(err) {
		err = os.MkdirAll("cache", 0755)
		if err != nil {
			log.Printf("Failed to create cache directory: %v", err)
		}
	}

	if cacheDurationToUse > 0 && (cacheBehaviorToUse == CacheBehaviorAll || cacheBehaviorToUse == CacheBehaviorRead) {
		isFresh, err := isCacheFresh(cacheFilePath, cacheDurationToUse)
		if err != nil {
			log.Printf("Error checking HTML cache freshness for '%s': %v", cacheFilePath, err)
		} else if isFresh {
			cachedData, readErr := ioutil.ReadFile(cacheFilePath)
			if readErr != nil {
				log.Printf("Error reading fresh HTML cache file '%s': %v", cacheFilePath, readErr)
			} else {
				c.Set("Content-Type", "text/html; charset=utf-8")
				if _, writeErr := c.Write(cachedData); writeErr != nil {
					log.Printf("Error writing cached HTML data to response: %v", writeErr)
					return writeErr
				}
				log.Printf("Served fresh HTML cache from '%s'", cacheFilePath)
				return nil
			}
		}

		if cacheBehaviorToUse == CacheBehaviorRead {
			log.Printf("Cache not fresh or usable for '%s', and behavior is 'read'. Not fetching from DB.", cacheFilePath)
			return c.Status(404).SendString("Cached data not available or not fresh.")
		}
	} else if cacheDurationToUse == 0 {
		log.Println("Cache duration is 0, skipping cache read for realtime data.")
	}

	log.Printf("Fetching fresh data for HTML cache from DB for query: %s", query)
	results, err := fetchDataFromDB(query, finalQueryParams...) 
	if err != nil {
		log.Printf("Error fetching data from database '%s': %v", selectedDBName, err)
		return c.Status(500).SendString(fmt.Sprintf("Database query error: %v", err))
	}

	var effectiveColumnOrder []string
	if columnOrder == nil || len(columnOrder) == 0 {
		if len(results) > 0 {
			for colName := range results[0] {
				effectiveColumnOrder = append(effectiveColumnOrder, colName)
			}
		} else {
			effectiveColumnOrder = []string{}
		}
	} else {
		effectiveColumnOrder = columnOrder
	}

	htmlTable, err := renderResultsToHtmlTable(results, effectiveColumnOrder)
	if err != nil {
		log.Printf("Error rendering HTML table: %v", err)
		return c.Status(500).SendString("Error rendering data to HTML table")
	}

	c.Set("Content-Type", "text/html; charset=utf-8")
	if _, err := c.Write([]byte(htmlTable)); err != nil {
		return err
	}

	if cacheDurationToUse > 0 && (cacheBehaviorToUse == CacheBehaviorAll || cacheBehaviorToUse == CacheBehaviorCreate) {
		err = ioutil.WriteFile(cacheFilePath, []byte(htmlTable), 0644)
		if err != nil {
			log.Printf("Error writing to HTML cache file '%s': %v", cacheFilePath, err)
		} else {
			log.Printf("Updated HTML cache file '%s'", cacheFilePath)
		}
	} else if cacheDurationToUse == 0 {
		log.Println("Cache duration is 0, skipping cache write for realtime data.")
	}

	return nil
}

func renderResultsToHtmlTable(results []map[string]interface{}, desiredOrder []string) (string, error) {
	if len(results) == 0 {
		return "<p>No data found.</p>", nil
	}

	if desiredOrder == nil || len(desiredOrder) == 0 {
		for colName := range results[0] {
			desiredOrder = append(desiredOrder, colName)
		}
	}

	var sb strings.Builder

	sb.WriteString("<table border=\"1\" style=\"width:100%; border-collapse: collapse;\"><thead><tr>")

	for _, colName := range desiredOrder {
		sb.WriteString("<th>")
		sb.WriteString(colName)
		sb.WriteString("</th>")
	}
	sb.WriteString("</tr></thead><tbody>")

	for _, row := range results {
		sb.WriteString("<tr>")
		for _, colName := range desiredOrder {
			sb.WriteString("<td>")
			val := row[colName]

			switch v := val.(type) {
			case time.Time:
				sb.WriteString(v.Format("2006-01-02 15:04:05"))
			case float64:
				sb.WriteString(fmt.Sprintf("%.0f", v))
			case nil:
				sb.WriteString("")
			default:
				sb.WriteString(template.HTMLEscapeString(fmt.Sprintf("%v", v)))
			}
			sb.WriteString("</td>")
		}
		sb.WriteString("</tr>")
	}
	sb.WriteString("</tbody></table>")

	return sb.String(), nil
}

func generateHtmlCacheFileName(dbName, query string, params ...interface{}) (string, error) {
	dataToHash := dbName + query
	for _, param := range params {
		dataToHash += fmt.Sprintf("%v", param)
	}
	hasher := sha256.New()
	hasher.Write([]byte(dataToHash))
	hash := hex.EncodeToString(hasher.Sum(nil))

	return fmt.Sprintf("html_query_%s.html", hash), nil
}

func GenericQueryHandler(c *fiber.Ctx, query string, opts ...interface{}) error {
	var cacheDurationToUse time.Duration = defaultCacheDuration
	var cacheBehaviorToUse string = CacheBehaviorAll
	var directQueryParams []interface{} 
	var cacheDurationProvided bool = false 

	httpContextParams := extractQueryParamsFromContext(c)
	for _, opt := range opts {
		switch v := opt.(type) {
		case int:
			if v == 0 {
				cacheDurationToUse = 0
				cacheBehaviorToUse = CacheBehaviorCreate 
			} else if v > 0 {
				cacheDurationToUse = time.Duration(v) * time.Minute
			}
		case string:
			switch strings.ToLower(v) {
			case CacheBehaviorRead:
				cacheBehaviorToUse = CacheBehaviorRead
			case CacheBehaviorCreate:
				cacheBehaviorToUse = CacheBehaviorCreate
			case CacheBehaviorAll:
				cacheBehaviorToUse = CacheBehaviorAll
			default:
				directQueryParams = append(directQueryParams, opt)
			}
		default:
			directQueryParams = append(directQueryParams, opt)
		}
	}


	var finalQueryParams []interface{}
	finalQueryParams = append(finalQueryParams, directQueryParams...)
	for _, p := range httpContextParams {
		finalQueryParams = append(finalQueryParams, p)
	}

	if query == "" {
		return c.Status(400).SendString("Query parameter cannot be empty")
	}

	if _, err := os.Stat("cache"); os.IsNotExist(err) {
		err = os.MkdirAll("cache", 0755)
		if err != nil {
			log.Printf("Failed to create cache directory: %v", err)
		}
	}

	cacheBaseName := fmt.Sprintf("json_%s", sanitizeFilename(query)) 
	if len(finalQueryParams) > 0 {
		paramStrings := make([]string, len(finalQueryParams))
		for i, p := range finalQueryParams {
			paramStrings[i] = fmt.Sprintf("%v", p)
		}
		cacheBaseName += "_" + sanitizeFilename(strings.Join(paramStrings, "_"))
	}
	cacheFileName := fmt.Sprintf("%s.json", cacheBaseName)
	cacheFilePath := filepath.Join("cache", cacheFileName)

    if !cacheDurationProvided && cacheDurationToUse == 0 {
        cacheDurationToUse = defaultCacheDuration
    }

	if cacheDurationToUse > 0 && (cacheBehaviorToUse == CacheBehaviorAll || cacheBehaviorToUse == CacheBehaviorRead) {
		isFresh, err := isCacheFresh(cacheFilePath, cacheDurationToUse)
		if err != nil {
			log.Printf("Error checking JSON cache freshness for '%s': %v", cacheFilePath, err)
		} else if isFresh {
			cachedData, readErr := ioutil.ReadFile(cacheFilePath)
			if readErr != nil {
				log.Printf("Error reading fresh JSON cache file '%s': %v", cacheFilePath, readErr)
			} else {
				c.Set("Content-Type", "application/json")
				if _, writeErr := c.Write(cachedData); writeErr != nil {
					log.Printf("Error writing cached data to response: %v", writeErr)
					return writeErr
				}
				log.Printf("Served fresh JSON cache from '%s'", cacheFilePath)
				return nil
			}
		}

		if cacheBehaviorToUse == CacheBehaviorRead {
			log.Printf("Cache not fresh or usable for '%s', and behavior is 'read'. Not fetching from DB.", cacheFilePath)
			return c.Status(404).SendString("Cached data not available or not fresh.")
		}
	} else if cacheDurationToUse == 0 {
		log.Println("Cache duration is 0, skipping cache read for realtime data.")
	}


	log.Printf("Fetching fresh data for JSON cache from DB for query: %s", query)
	results, err := fetchDataFromDB(query, finalQueryParams...) 
	if err != nil {
		log.Printf("Error fetching data from database: %v", err)
		return c.Status(500).SendString(fmt.Sprintf("Database query error: %v", err))
	}

	jsonData, err := json.Marshal(results)
	if err != nil {
		log.Printf("Error marshaling JSON: %v", err)
		return c.Status(500).SendString("Error marshaling data to JSON")
	}

	c.Set("Content-Type", "application/json")
	if _, err := c.Write(jsonData); err != nil {
		return err
	}

	if cacheDurationToUse > 0 && (cacheBehaviorToUse == CacheBehaviorAll || cacheBehaviorToUse == CacheBehaviorCreate) {
		err = ioutil.WriteFile(cacheFilePath, jsonData, 0644)
		if err != nil {
			log.Printf("Error writing to cache file '%s': %v", cacheFilePath, err)
		} else {
			log.Printf("Updated JSON cache file '%s'", cacheFilePath)
		}
	} else if cacheDurationToUse == 0 {
		log.Println("Cache duration is 0, skipping cache write for realtime data.")
	}

	return nil
}

func fetchDataFromDB(query string, params ...interface{}) ([]map[string]interface{}, error) {
	var rows *sql.Rows
	var err error

	var actualDBParams []interface{}
	for _, p := range params {
		switch p.(type) {
		case string, int, int64, float64, bool, time.Time, nil:
			actualDBParams = append(actualDBParams, p)
		default:
			log.Printf("Warning: Skipping unsupported parameter type for DB query: %T %v", p, p)
		}
	}

	rows, err = db.DB.Query(query, actualDBParams...)

	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	results := []map[string]interface{}{}
	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	for rows.Next() {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}

		rowMap := make(map[string]interface{})
		for i, colName := range cols {
			val := columns[i]

			if val == nil {
				rowMap[colName] = nil
				continue
			}

			switch v := val.(type) {
			case []byte:
				strVal := string(v)
				if numVal, err := strconv.ParseFloat(strVal, 64); err == nil {
					rowMap[colName] = numVal
				} else {
					rowMap[colName] = strVal
				}
			case int, int8, int16, int32, int64:
				rowMap[colName] = v
			case uint, uint8, uint16, uint32, uint64:
				rowMap[colName] = v
			case float32, float64:
				rowMap[colName] = v
			case bool:
				rowMap[colName] = v
			case time.Time:
				rowMap[colName] = v.Format(time.RFC3339)
			case string:
				if numVal, err := strconv.ParseFloat(v, 64); err == nil {
					rowMap[colName] = numVal
				} else {
					rowMap[colName] = v
				}
			default:
				rowMap[colName] = v
			}
		}
		results = append(results, rowMap)
	}
	return results, nil
}

func generateCacheFileName(query string, params ...interface{}) (string, error) {

	dataToHash := query
	for _, param := range params {
		dataToHash += fmt.Sprintf("%v", param)
	}

	hasher := sha256.New()
	hasher.Write([]byte(dataToHash))
	hash := hex.EncodeToString(hasher.Sum(nil))

	return fmt.Sprintf("query_%s.json", hash), nil
}

func sanitizeFilename(filename string) string {
    filename = strings.ReplaceAll(filename, "\n", " ") 
    filename = strings.ReplaceAll(filename, "\r", " ")
    filename = strings.Join(strings.Fields(filename), "_") 

    filename = strings.ReplaceAll(filename, "/", "_")
    filename = strings.ReplaceAll(filename, "\\", "_")
    filename = strings.ReplaceAll(filename, ":", "_")
    filename = strings.ReplaceAll(filename, "*", "_")
    filename = strings.ReplaceAll(filename, "?", "_")
    filename = strings.ReplaceAll(filename, "\"", "_")
    filename = strings.ReplaceAll(filename, "<", "_")
    filename = strings.ReplaceAll(filename, ">", "_")
    filename = strings.ReplaceAll(filename, "|", "_")
    filename = strings.ReplaceAll(filename, "%", "_")
    filename = strings.ReplaceAll(filename, "'", "")  
    filename = strings.ReplaceAll(filename, "`", "") 
    filename = strings.ReplaceAll(filename, "(", "") 
    filename = strings.ReplaceAll(filename, ")", "")
    filename = strings.ReplaceAll(filename, "[", "") 
    filename = strings.ReplaceAll(filename, "]", "")
    filename = strings.ReplaceAll(filename, "{", "")  
    filename = strings.ReplaceAll(filename, "}", "")
    filename = strings.ReplaceAll(filename, ",", "") 
    filename = strings.ReplaceAll(filename, ";", "") 
    filename = strings.ReplaceAll(filename, "=", "_") 
    filename = strings.ReplaceAll(filename, "--", "_") 

    if len(filename) > 100 { 
        hashSuffix := generateHashSuffix(filename)
        filename = filename[:(100 - len(hashSuffix) - 1)] + "_" + hashSuffix
    }

    for strings.Contains(filename, "__") {
        filename = strings.ReplaceAll(filename, "__", "_")
    }

    filename = strings.Trim(filename, "_")
    return filename
}

func isCacheFresh(filePath string, duration time.Duration) (bool, error) {
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("error getting file info for '%s': %w", filePath, err)
	}

	return time.Since(fileInfo.ModTime()) < duration, nil
}

func generateHashSuffix(input string) string {
	hasher := sha256.New()
	hasher.Write([]byte(input))
	return hex.EncodeToString(hasher.Sum(nil))[:8] 
}

func extractQueryParamsFromContext(c *fiber.Ctx) []string {
	var combinedParams []string
	c.Request().URI().QueryArgs().VisitAll(func(key, value []byte) {
		combinedParams = append(combinedParams, fmt.Sprintf("%s=%s", key, value))
	})

	if c.Method() == "POST" || c.Method() == "PUT" || c.Method() == "PATCH" {
		contentType := string(c.Request().Header.ContentType())
		if strings.HasPrefix(contentType, "application/json") {
			var jsonBody map[string]interface{}
			if err := json.Unmarshal(c.Body(), &jsonBody); err == nil {
				for k, v := range jsonBody {
					combinedParams = append(combinedParams, fmt.Sprintf("%s=%v", k, v))
				}
			} else {
				log.Printf("Warning: Could not parse JSON body for cache naming: %v", err)
			}
		} else if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") || strings.HasPrefix(contentType, "multipart/form-data") {
			c.Request().PostArgs().VisitAll(func(key, value []byte) {
				combinedParams = append(combinedParams, fmt.Sprintf("%s=%s", key, value))
			})
		}
	}

	return combinedParams
}