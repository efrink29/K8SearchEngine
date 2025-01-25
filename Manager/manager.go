package main

import (
	"context"

	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v4"
)

var db1 *pgx.Conn

func initDB() {
	var err error

	// Connect to the first database
	db1ConnStr := os.Getenv("DB1_CONN")
	db1, err := pgx.Connect(context.Background(), db1ConnStr)
	if err != nil {
		fmt.Printf("Error connecting to database: %v\n", err)
		return
	}
	defer db1.Close(context.Background())

}

// Handle database health checks
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	err1 := db1.Ping(context.Background())

	if err1 != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

	w.Write([]byte("All systems operational"))
}

// Forward work to crawler nodes
func forwardWorkHandler(w http.ResponseWriter, r *http.Request) {
	crawlers := []string{
		os.Getenv("CRAWLER_1"),
		os.Getenv("CRAWLER_2"),
		os.Getenv("CRAWLER_3"),
	}

	for _, crawler := range crawlers {
		resp, err := http.Get(fmt.Sprintf("http://%s/process", crawler))
		if err != nil {
			log.Printf("Error communicating with crawler %s: %v", crawler, err)
			continue
		}
		defer resp.Body.Close()
		log.Printf("Crawler %s responded with status: %s", crawler, resp.Status)
	}

	w.Write([]byte("Work forwarded to crawlers"))
}

func main() {
	// Initialize database connections
	initDB()
	defer db1.Close(context.Background())

	// Define HTTP handlers
	http.HandleFunc("/health", healthCheckHandler)
	http.HandleFunc("/work", forwardWorkHandler)

	// Start the manager server
	port := os.Getenv("MANAGER_PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Manager running on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
