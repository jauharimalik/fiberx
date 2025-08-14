package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/denisenkom/go-mssqldb" // Driver SQL Server
)

var DB *sql.DB // Global variable for the database connection, exported as 'DB'

// DefaultDBName holds the default database name for connections.
// It is exported (starts with a capital letter) so it can be accessed from other packages.
var DefaultDBName = "PK_ANP_DEV_QUERY" // Example default database name

// DBs is a map to potentially manage multiple database connections.
// For this specific application, we are primarily using the single global 'DB' connection.
var DBs = map[string]*sql.DB{}

// Connect initializes the connection to the SQL Server database.
// This function should be called once at the application's startup.
func Connect() {
	var err error
	
	// Define your connection string for the SQL Server database.
	// connectionString := "server=192.168.60.14;user id=pk;password=pk@query123!!!;database=DB_PGR;encrypt=disable"

	connectionString := "server=192.168.100.202;user id=PK-SERVE;password=n0v@0707#;database=PK_ANP_DEV_QUERY;encrypt=disable"
	// Open a database connection using the "sqlserver" driver.
	DB, err = sql.Open("sqlserver", connectionString)
	if err != nil {
		// If connection fails, log a fatal error and exit.
		log.Fatalf("Database connection failed: %v", err)
	}

	// Ping the database to verify that the connection is alive and valid.
	err = DB.Ping()
	if err != nil {
		// If ping fails, log a fatal error and exit.
		log.Fatalf("Database ping failed: %v", err)
	}

	fmt.Println("Successfully connected to SQL Server")

	// Store the primary DB connection in the DBs map using the DefaultDBName.
	// This makes it accessible via db.DBs[db.DefaultDBName] if needed,
	// though direct access to db.DB is also possible.
	DBs[DefaultDBName] = DB
}

// GetDB returns the initialized database connection instance.
// This function provides a way to retrieve the global DB connection.
func GetDB() *sql.DB {
	// Check if the database connection has been initialized.
	if DB == nil {
		log.Fatal("Database connection not initialized. Call db.Connect() first.")
	}
	return DB // Return the global DB connection.
}

// CloseDB closes the database connection.
// This should be called before the application exits to release database resources.
func CloseDB() {
	// Check if the database connection is open before attempting to close.
	if DB != nil {
		err := DB.Close()
		if err != nil {
			// Log any errors encountered while closing the connection.
			log.Printf("Error closing database connection: %v", err)
		}
		fmt.Println("Database connection closed.")
	}
}
