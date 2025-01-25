package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v4"
	"golang.org/x/net/html"
)

type ReverseIndex map[string][]UrlFrequency // map of word to list of URLs and their frequency

type WordFrequency struct {
	word      string
	frequency int
}

type UrlFrequency struct {
	url       string
	frequency int
}

func main() {
	//links := &messagepackage.Links{Links: []string{}} // Initialize Links struct

	startURL := os.Args[1]
	connStr := os.Args[2] // PostgreSQL connection string
	visited := make(map[string]bool)
	reverseIndex := make(ReverseIndex)

	// Initialize database connection
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		fmt.Printf("Error connecting to database: %v\n", err)
		return
	}
	defer conn.Close(context.Background())

	crawl(startURL, visited, reverseIndex, 0)

	// Save index to file
	file, err := os.Create("index.txt")
	if err != nil {
		fmt.Printf("Error creating index file: %v\n", err)
		return
	}
	defer file.Close()

	// Sort each list of URLs for each word by frequency
	for word, urls := range reverseIndex {
		// Sort URLs by frequency in descending order
		for i := 0; i < len(urls); i++ {
			for j := i + 1; j < len(urls); j++ {
				if urls[i].frequency < urls[j].frequency {
					urls[i], urls[j] = urls[j], urls[i]
				}
			}
		}
		reverseIndex[word] = urls

	}

	for word, urls := range reverseIndex {

		for _, urlFreq := range urls {
			file.WriteString(fmt.Sprintf("%s: %s - %d\n", word, urlFreq.url, urlFreq.frequency))
		}
	}

	// Ensure the table exists
	err = initializeWordDB(conn)
	if err != nil {
		fmt.Printf("Error initializing word database: %v\n", err)
		return
	}

	// Ensure the URL table exists
	err = initializeURLDB(conn)
	if err != nil {
		fmt.Printf("Error initializing URL database: %v\n", err)
		return
	}

	for url := range visited {
		err = saveURLToDB(conn, url)
		if err != nil {
			fmt.Printf("Error saving URL %s to database: %v\n", url, err)
			return
		}
	}

	// Save index to the database
	//err = saveIndexToDB(conn, reverseIndex)
	if err != nil {
		fmt.Printf("Error saving index to database: %v\n", err)
		return
	}

	fmt.Println("Index successfully saved to the database.")
}

func initializeURLDB(conn *pgx.Conn) error {
	createTableQuery := `
		CREATE TABLE IF NOT EXISTS url_index (
			url TEXT PRIMARY KEY
		);
	`
	_, err := conn.Exec(context.Background(), createTableQuery)
	return err
}

func saveURLToDB(conn *pgx.Conn, url string) error {
	_, err := conn.Exec(context.Background(), `
		INSERT INTO url_index (url)
		VALUES ($1)
		ON CONFLICT (url) DO NOTHING;
	`, url)
	return err
}

func initializeWordDB(conn *pgx.Conn) error {
	createTableQuery := `
		CREATE TABLE IF NOT EXISTS word_index (
			word TEXT NOT NULL,
			url TEXT NOT NULL,
			frequency INT NOT NULL,
			PRIMARY KEY (word, url)
		);
	`
	_, err := conn.Exec(context.Background(), createTableQuery)
	return err
}

func saveIndexToDB(conn *pgx.Conn, index ReverseIndex) error {
	for word, urls := range index {
		for _, urlFreq := range urls {
			_, err := conn.Exec(context.Background(), `
				INSERT INTO word_index (word, url, frequency)
				VALUES ($1, $2, $3)
				ON CONFLICT (word, url) DO UPDATE
				SET frequency = word_index.frequency + EXCLUDED.frequency;
			`, word, urlFreq.url, urlFreq.frequency)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func crawl(url string, visited map[string]bool, index ReverseIndex, depth int) {
	if visited[url] {
		return
	}
	visited[url] = true

	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != 200 {
		fmt.Printf("Error crawling %s: %v\n", url, err)
		return
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	// Remove Headers, Footers, and other non-content nodes

	if err != nil {
		fmt.Printf("Error parsing HTML for %s: %v\n", url, err)
		return
	}

	fmt.Printf("Crawling %s \n", url)

	// Extract text and tokenize
	text := extractText(doc)
	// Open file
	file, err := os.Create("output.txt")
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	_, err = file.WriteString(text)
	if err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
	}
	file.Close()

	tokens := tokenize(text)

	// Update reverse index
	for _, token := range tokens {
		if _, ok := index[token]; !ok {
			newUrl := UrlFrequency{url, 1}
			index[token] = []UrlFrequency{newUrl}
		}
		detectedUrl := false
		for i, urlFrequency := range index[token] {
			if urlFrequency.url == url {
				index[token][i].frequency++
				detectedUrl = true
				break
			}
		}
		if !detectedUrl {
			newUrl := UrlFrequency{url, 1}
			index[token] = append(index[token], newUrl)
		}
	}

	// Extract links and crawl them
	if depth > 0 {
		links := extractLinks(doc, url)
		linkCount := 0
		for _, link := range links {
			if linkCount >= 50 {
				break
			}
			crawl(link, visited, index, depth-1)
			linkCount++
		}
	}
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
					if !strings.Contains(normalizedLink, "wikipedia") {
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
