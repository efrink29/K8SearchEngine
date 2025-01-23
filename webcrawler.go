package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

type ReverseIndex map[string][]UrlFrequency // map of word to list of URLs and their frequency

type UrlFrequency struct {
	url       string
	frequency int
}

func main() {
	startURL := os.Args[1]
	visited := make(map[string]bool)
	reverseIndex := make(ReverseIndex)

	crawl(startURL, visited, reverseIndex, 3)
	file, err := os.Create(os.Args[2])
	if err != nil {
		fmt.Printf("Error creating index file: %v\n", err)
		return
	}
	for word, urls := range reverseIndex {
		file.WriteString(word + ": ")
		for _, url := range urls {
			file.WriteString(url.url + " " + fmt.Sprint(url.frequency) + " | ")
		}
		file.WriteString("\n")

	}
	file.Close()
	fmt.Println("Index written to index.txt")
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
	if err != nil {
		fmt.Printf("Error parsing HTML for %s: %v\n", url, err)
		return
	}

	fmt.Printf("Crawling %s \n", url)

	// Extract text and tokenize
	text := extractText(doc)
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
			if linkCount >= 5 {
				break
			}
			crawl(link, visited, index, depth-1)
			linkCount++
		}
	}
}

func extractText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "head" || n.Data == "noscript" || n.Data == "svg" || n.Data == "img" || n.Data == "iframe") {
		return ""
	}

	var text string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		text += extractText(c)
	}

	return strings.TrimSpace(text)
}

func tokenize(text string) []string {
	re := regexp.MustCompile(`([A-Z]??[a-z]*)|([A-Z]+)`)
	words := re.FindAllString(strings.ToLower(text), -1)
	return words
}

func extractLinks(doc *html.Node, baseURL string) []string {
	var links []string
	var extract func(*html.Node)

	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		fmt.Printf("Error parsing base URL %s: %v\n", baseURL, err)
		return links
	}

	extract = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					link := attr.Val
					parsedLink, err := parsedBase.Parse(link)
					if err != nil {
						continue
					}
					normalizedLink := parsedLink.String()
					normalizedLink = strings.TrimSpace(normalizedLink)
					normalizedLink = strings.TrimSuffix(normalizedLink, "/")
					if !contains(links, normalizedLink) && (strings.Contains(normalizedLink, "https://www.") || strings.Contains(normalizedLink, "https://en.")) && !strings.Contains(normalizedLink, "#") && !contains(links, strings.TrimSuffix(normalizedLink, "/")) {
						links = append(links, normalizedLink)
					}
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
