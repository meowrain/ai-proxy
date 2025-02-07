package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Port       string            `json:"port"`
	APIMapping map[string]string `json:"api_mapping"`
}

var configuration Config

func init() {
	log.Println("Load Configuration Successfully")
	readConfig()
}

func main() {
	http.HandleFunc("/", handler)
	log.Printf("Server started at :%s", configuration.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", configuration.Port), nil))
}

func readConfig() {
	file, err := os.Open("api.json")
	if err != nil {
		log.Fatalf("Failed to open JSON file: %v", err)
	}
	defer file.Close()
	bytes, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("Failed to read JSON file: %v", err)
	}

	if err := json.Unmarshal(bytes, &configuration); err != nil {
		log.Fatalf("Failed to parse JSON: %v", err)
	}
	for prefix, target := range configuration.APIMapping {
		if prefix == "" || target == "" {
			log.Fatalf("Invalid API mapping: prefix=%s, target=%s", prefix, target)
		}
	}
}
func handler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Incoming request: %s %s", r.Method, r.URL.Path)

	switch r.URL.Path {
	case "/", "/index.html":
		writeResponse(w, http.StatusOK, "text/html", "Service is running!")
		return
	case "/robots.txt":
		writeResponse(w, http.StatusOK, "text/plain", "User-agent: *\nDisallow: /")
		return
	}

	prefix, rest := extractPrefixAndRest(r.URL.Path)
	if prefix == "" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	targetBase := configuration.APIMapping[prefix]
	query := r.URL.RawQuery
	if query != "" {
		query = "?" + query
	}
	targetURL := targetBase + rest + query

	log.Printf("Matched prefix: %s, Rest path: %s, Target URL: %s", prefix, rest, targetURL)
	forwardRequest(w, r, targetURL)
}

func forwardRequest(w http.ResponseWriter, r *http.Request, targetURL string) {
	target, err := url.Parse(targetURL)
	if err != nil {
		log.Printf("Failed to parse target URL: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Director = func(req *http.Request) {
		req.Method = r.Method
		req.Host = target.Host // 关键修复点
		req.URL = target
		log.Printf("Forwarding request to: %s", req.URL)
		req.Header = r.Header.Clone()
		req.Body = r.Body
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		setSecurityHeaders(resp.Header)
		return nil
	}

	proxy.ServeHTTP(w, r)
}

func setSecurityHeaders(header http.Header) {
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
	header.Set("Referrer-Policy", "no-referrer")
}

func writeResponse(w http.ResponseWriter, statusCode int, contentType, body string) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(statusCode)
	w.Write([]byte(body))
}

func extractPrefixAndRest(pathname string) (string, string) {
	var matchedPrefix string
	var matchedRest string

	// 遍历所有前缀，找到最长的匹配前缀
	for prefix := range configuration.APIMapping {
		if strings.HasPrefix(pathname, prefix) {
			// 如果当前匹配的前缀比之前匹配的前缀更长，则更新
			if len(prefix) > len(matchedPrefix) {
				matchedPrefix = prefix
				matchedRest = pathname[len(prefix):]
			}
		}
	}

	return matchedPrefix, matchedRest
}
