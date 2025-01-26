package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

var (
	linkQueue = make(chan string, 1000)
	workers   = 5 // Number of workers to process links concurrently
	mu        sync.Mutex
)

type Link struct {
	URL string `json:"url"`
}

func main() {
	// Read environment variables
	connStrDB := os.Getenv("DB_CONN_STRING")
	crawlerService := os.Getenv("CRAWLER_SERVICE_HOST")
	if crawlerService == "" {
		log.Fatal("CRAWLER_SERVICE_HOST environment variable not set")
	}

	// Initialize database connection pool
	pool, err := pgxpool.Connect(context.Background(), connStrDB)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer pool.Close()

	// Start link processors
	for i := 0; i < workers; i++ {
		go processLinks(pool, crawlerService)
	}

	// HTTP handlers
	http.HandleFunc("/enqueue", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		var link Link
		if err := json.NewDecoder(r.Body).Decode(&link); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		enqueueLink(link.URL)
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("Link enqueued"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	log.Printf("Manager service running on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func enqueueLink(url string) {
	mu.Lock()
	defer mu.Unlock()
	select {
	case linkQueue <- url:
		log.Printf("Link enqueued: %s", url)
	default:
		log.Printf("Queue full, dropping link: %s", url)
	}
}

func processLinks(pool *pgxpool.Pool, crawlerService string) {
	for link := range linkQueue {
		log.Printf("Processing link: %s", link)

		// Forward link to crawler
		err := forwardLinkToCrawler(crawlerService, link)
		if err != nil {
			log.Printf("Error forwarding link to crawler: %v", err)
			continue
		}

		// Save link to database
		err = saveLinkToDB(context.Background(), pool, link)
		if err != nil {
			log.Printf("Error saving link to database: %v", err)
		}
	}
}

func forwardLinkToCrawler(crawlerService, link string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("http://%s/crawl", crawlerService), nil)
	if err != nil {
		return err
	}

	q := req.URL.Query()
	q.Add("url", link)
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("crawler service returned status %d", resp.StatusCode)
	}

	return nil
}

func saveLinkToDB(ctx context.Context, pool *pgxpool.Pool, link string) error {
	_, err := pool.Exec(ctx, "INSERT INTO links (url, created_at) VALUES ($1, NOW())", link)
	return err
}
