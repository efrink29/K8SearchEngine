package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"golang.org/x/net/html"
)

type ReverseIndex map[string]int

func main() {
	// Read environment variables
	connStrDB := os.Getenv("DB1_CONN")
	managerService := os.Getenv("MANAGER_SERVICE_HOST")
	if managerService == "" {
		log.Fatal("MANAGER_SERVICE_HOST environment variable not set")
	}

	// Initialize database connection pool
	log.Printf("Connecting to database %s", connStrDB)
	pool, err := pgxpool.Connect(context.Background(), connStrDB)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer pool.Close()

	// Start HTTP server
	http.HandleFunc("/crawl", func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Query().Get("url")
		if url == "" {
			http.Error(w, "Missing URL parameter", http.StatusBadRequest)
			return
		}

		go crawl(url, pool, managerService)
		w.Write([]byte("Crawling started"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Starting server on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func crawl(pageURL string, database *pgxpool.Pool, managerService string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := http.Get(pageURL)
	if err != nil || resp.StatusCode != 200 {
		log.Printf("Error crawling %s: %v", pageURL, err)
		return
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		log.Printf("Error parsing HTML for %s: %v", pageURL, err)
		return
	}

	log.Printf("Crawling %s", pageURL)

	// Extract metadata
	title, description := extractMetadata(doc)

	// Save metadata to database
	err = saveMetaDataToDB(ctx, database, pageURL, title, description)
	if err != nil {
		log.Printf("Error saving metadata for %s: %v", pageURL, err)
		return
	}

	// Extract text and tokenize
	text := extractText(doc)
	tokens := tokenize(text)
	index := make(ReverseIndex)

	for _, token := range tokens {
		index[token]++
	}

	// Save reverse index to database
	err = saveIndexToDB(ctx, database, index, pageURL)
	if err != nil {
		log.Printf("Error saving index for %s: %v", pageURL, err)
		return
	}

	// Extract and normalize links
	links := extractLinks(doc, pageURL)
	for _, link := range links {
		sendLinkToManager(managerService, link)
	}
}

func sendLinkToManager(managerService, link string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://%s/crawl?url=%s", managerService, url.QueryEscape(link)), nil)
	if err != nil {
		log.Printf("Error creating request for link %s: %v", link, err)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error forwarding link %s to manager: %v", link, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Manager service returned status %d for link %s", resp.StatusCode, link)
	}
}

func saveMetaDataToDB(ctx context.Context, conn *pgxpool.Pool, url, title, description string) error {
	_, err := conn.Exec(ctx, "INSERT INTO metadata (url, title, description) VALUES ($1, $2, $3)", url, title, description)
	return err
}

func saveIndexToDB(ctx context.Context, conn *pgxpool.Pool, index ReverseIndex, url string) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Insert words into the words table
	for word := range index {
		_, err := tx.Exec(ctx, "INSERT INTO words (word) VALUES ($1) ON CONFLICT DO NOTHING", word)
		if err != nil {
			return err
		}
	}

	// Insert index into the reverse index table
	for word, count := range index {
		_, err := tx.Exec(ctx, "INSERT INTO reverse_index (word_id, url, count) VALUES ((SELECT id FROM words WHERE word = $1), $2, $3)", word, url, count)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func extractMetadata(doc *html.Node) (string, string) {
	var title, description string

	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" {
			title = extractText(n)
		}
		if n.Type == html.ElementNode && n.Data == "meta" {
			var name, content string
			for _, attr := range n.Attr {
				if attr.Key == "name" {
					name = attr.Val
				}
				if attr.Key == "content" {
					content = attr.Val
				}
			}
			if name == "description" {
				description = content
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}

	extract(doc)
	return title, description
}

func extractText(n *html.Node) string {
	if n.Type == html.TextNode {

		return strings.ToLower(n.Data)
	}
	if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "head" || n.Data == "noscript" || n.Data == "svg" || n.Data == "img" || n.Data == "iframe") {
		return ""
	}

	var text string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		text += extractText(c) + " "
	}

	return strings.TrimSpace(text)
}

func tokenize(text string) []string {
	re := regexp.MustCompile(`([A-Z]??[a-z]+)|([A-Z]+)`)
	words := re.FindAllString(strings.ToLower(text), -1)
	return words
}

func extractLinks(doc *html.Node, baseURL string) []string {
	var links []string

	// Parse the base URL
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		fmt.Printf("Error parsing base URL %s: %v\n", baseURL, err)
		return links
	}

	// Regular expression to match banned substrings
	bannedPattern := regexp.MustCompile(`(?i)(Account|Login|Logout|Sign|Register|Contact|Special|Module|Template|Help|Talk|User|wikidata|wikibooks|wikiversity|wikinews|wikivoyage|php|mediawiki|wikiquote|wiktionary|File|Category|Portal|#)`)

	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					link := attr.Val

					// Parse and normalize the link
					parsedLink, err := parsedBase.Parse(link)
					if err != nil {
						continue
					}
					normalizedLink := strings.TrimSuffix(strings.TrimSpace(parsedLink.String()), "/")
					if !strings.Contains(normalizedLink, "https://en.wikipedia.org/wiki/") {
						continue
					}
					// Skip if the link is banned, already added, or doesn't match base domains
					if bannedPattern.MatchString(normalizedLink) || contains(links, normalizedLink) ||
						!( /*strings.Contains(normalizedLink, "https://www.") || strings.Contains(normalizedLink, "https://en.") ||*/ strings.Contains(normalizedLink, "https://en.wikipedia.org/wiki/")) {
						continue
					}

					// Add valid link to the list
					links = append(links, normalizedLink)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}

	extract(doc)
	return links
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
