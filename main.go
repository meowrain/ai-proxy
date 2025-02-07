package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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

}
func handler(w http.ResponseWriter, r *http.Request) {
	pathname := r.URL.Path
	log.Printf("Incoming request: %s %s", r.Method, pathname)

	switch pathname {
	case "/", "/index.html":
		log.Println("Handling root or index request")
		writeResponse(w, http.StatusOK, "text/html", "Service is running!")
		return
	case "/robots.txt":
		log.Println("Handling robots.txt request")
		writeResponse(w, http.StatusOK, "text/plain", "User-agent: *\nDisallow: /")
		return
	}

	prefix, rest := extractPrefixAndRest(pathname)
	if prefix == "" {
		log.Printf("No matching prefix found for path: %s", pathname)
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	targetURL := configuration.APIMapping[prefix] + rest
	log.Printf("Forwarding request to: %s", targetURL)
	forwardRequest(w, r, targetURL)
}

func forwardRequest(w http.ResponseWriter, r *http.Request, targetURL string) {
	log.Printf("Creating new request to target URL: %s", targetURL)
	req, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		http.Error(w, "Failed to create request: "+err.Error(), http.StatusInternalServerError)
		return
	}

	copyHeaders(r.Header, req.Header, []string{"Accept", "Content-Type", "Authorization"})

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to forward request: %v", err)
		http.Error(w, "Failed to forward request: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	log.Printf("Received response from target URL: %s, Status: %s", targetURL, resp.Status)
	setSecurityHeaders(w)
	copyHeaders(resp.Header, w.Header(), nil)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func copyHeaders(src http.Header, dest http.Header, allowedHeaders []string) {
	if allowedHeaders == nil {
		for key, values := range src {
			for _, value := range values {
				dest.Add(key, value)
			}
		}
		return
	}

	for _, h := range allowedHeaders {
		if val := src.Get(h); val != "" {
			dest.Set(h, val)
		}
	}
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

func writeResponse(w http.ResponseWriter, statusCode int, contentType, body string) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(statusCode)
	w.Write([]byte(body))
}

func extractPrefixAndRest(pathname string) (string, string) {
	for prefix := range configuration.APIMapping {
		if strings.HasPrefix(pathname, prefix) {
			return prefix, pathname[len(prefix):]
		}
	}
	return "", ""
}
