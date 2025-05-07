package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"golang.org/x/net/proxy"
)

type ProxyConfig struct {
	Type    string `json:"type"`
	Address string `json:"address,omitempty"` // Preferred field for proxy address
	URL     string `json:"url,omitempty"`     // Alternative field, used if Address is empty
}

// GetAddress returns the proxy address, preferring Address field over URL field.
func (pc *ProxyConfig) GetAddress() string {
	if pc.Address != "" {
		return pc.Address
	}
	return pc.URL
}

type TargetConfig struct {
	// TargetURL will be populated if the json value is an object with a "target_url" field.
	TargetURL string       `json:"target_url,omitempty"`
	Proxy     *ProxyConfig `json:"proxy,omitempty"`
	// resolvedTargetURL stores the actual target URL, whether from a direct string or "target_url" field.
	resolvedTargetURL string
}

// UnmarshalJSON allows TargetConfig to be either a string (URL) or an object {target_url: "", proxy: {}}.
func (tc *TargetConfig) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as a simple string first (for backward compatibility)
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		tc.resolvedTargetURL = s
		// tc.TargetURL = s // Optionally populate TargetURL too for direct access, though resolvedTargetURL is primary
		return nil
	}

	// If it's not a string, try to unmarshal as an object.
	// Use an alias type to avoid recursion with UnmarshalJSON.
	type Alias TargetConfig
	var aux struct {
		Alias
	} // Use a struct to ensure all fields of Alias are unmarshalled

	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal TargetConfig as object: %w", err)
	}

	*tc = TargetConfig(aux.Alias) // Assign the unmarshalled object fields, with explicit conversion
	if aux.TargetURL == "" {      // If object form is used, target_url is mandatory within it
		// This check is important if the string unmarshal failed not because it's an object,
		// but because it's an invalid JSON value altogether.
		// However, if unmarshal to string fails, and unmarshal to aux (object) succeeds,
		// then aux.TargetURL must be the intended URL.
		// If aux.TargetURL is empty here, it means JSON was like {"proxy": {...}} without target_url
		if tc.resolvedTargetURL == "" { // Check if it was not set by string parsing attempt
			// If resolvedTargetURL is also empty, it means we didn't even get a string.
			// And if aux.TargetURL is empty, it means the object form also didn't have target_url
			return fmt.Errorf("target_url is required when api_mapping value is an object, or the value must be a simple string URL")
		}
		// If resolvedTargetURL is not empty, it means string parsing failed but we might have a valid proxy object
		// without its own target_url, intending to use a global one perhaps (not supported directly this way).
		// The current logic implies that if it's an object, "target_url" is key.
		// If tc.resolvedTargetURL was set by a failed string parse that was actually an object,
		// this path is complex. Simpler: if we are in object-parsing mode, aux.TargetURL is king.
	} else {
		tc.resolvedTargetURL = aux.TargetURL
	}

	return nil
}

// GetActualTargetURL returns the definitive target URL.
func (tc *TargetConfig) GetActualTargetURL() string {
	return tc.resolvedTargetURL
}

type Config struct {
	Port        string                  `json:"port"`
	APIMapping  map[string]TargetConfig `json:"api_mapping"` // Value is now TargetConfig
	GlobalProxy *ProxyConfig            `json:"proxy,omitempty"`
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

	// Validate Global Proxy if present
	if cfgProxy := configuration.GlobalProxy; cfgProxy != nil {
		proxyAddress := cfgProxy.GetAddress()
		if (cfgProxy.Type != "http" && cfgProxy.Type != "socks5") || proxyAddress == "" {
			log.Fatalf("Invalid global proxy configuration: type=%s, address/url=%s", cfgProxy.Type, proxyAddress)
		}
		if cfgProxy.Type == "http" {
			if _, err := url.ParseRequestURI(proxyAddress); err != nil {
				log.Fatalf("Invalid global HTTP proxy address/url: %s. It must be a valid URI. Error: %v", proxyAddress, err)
			}
		}
	}

	for prefix, targetCfg := range configuration.APIMapping {
		actualURL := targetCfg.GetActualTargetURL()
		if prefix == "" || actualURL == "" {
			log.Fatalf("Invalid API mapping: prefix=%s, target_url cannot be empty. Parsed TargetConfig: %+v", prefix, targetCfg)
		}
		if targetCfg.Proxy != nil {
			proxyAddress := targetCfg.Proxy.GetAddress()
			if (targetCfg.Proxy.Type != "http" && targetCfg.Proxy.Type != "socks5") || proxyAddress == "" {
				log.Fatalf("Invalid proxy configuration for prefix %s: type=%s, address/url=%s", prefix, targetCfg.Proxy.Type, proxyAddress)
			}
			// Ensure http proxy address is a valid URL
			if targetCfg.Proxy.Type == "http" {
				if _, err := url.ParseRequestURI(proxyAddress); err != nil {
					log.Fatalf("Invalid HTTP proxy address/url for prefix %s: %s. It must be a valid URI. Error: %v", prefix, proxyAddress, err)
				}
			}
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

	targetConfig, ok := configuration.APIMapping[prefix]
	if !ok {
		http.Error(w, "Configuration error: No mapping for prefix", http.StatusInternalServerError)
		return
	}

	// Determine effective proxy: specific API proxy > global proxy > no proxy
	var effectiveProxyCfg *ProxyConfig
	if targetConfig.Proxy != nil {
		effectiveProxyCfg = targetConfig.Proxy
	} else if configuration.GlobalProxy != nil {
		effectiveProxyCfg = configuration.GlobalProxy
	}

	targetBase := targetConfig.GetActualTargetURL()
	if targetBase == "" { // Should be caught by readConfig, but as a safeguard
		log.Printf("Error: Target base URL is empty for prefix %s", prefix)
		http.Error(w, "Internal Server Error: Misconfigured target", http.StatusInternalServerError)
		return
	}
	query := r.URL.RawQuery
	if query != "" {
		query = "?" + query
	}
	targetURL := targetBase + rest + query

	log.Printf("Matched prefix: %s, Rest path: %s, Target URL: %s", prefix, rest, targetURL)
	forwardRequest(w, r, targetURL, effectiveProxyCfg)
}

func forwardRequest(w http.ResponseWriter, r *http.Request, targetURL string, proxyCfg *ProxyConfig) {
	target, err := url.Parse(targetURL)
	if err != nil {
		log.Printf("Failed to parse target URL: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(target)

	if proxyCfg != nil {
		proxyAddress := proxyCfg.GetAddress()
		log.Printf("Using proxy for target %s: Type=%s, Address/URL=%s", targetURL, proxyCfg.Type, proxyAddress)
		transport := &http.Transport{}
		switch proxyCfg.Type {
		case "http":
			// Ensure proxyAddress is a full URL for HTTP proxy, e.g., "http://proxy.example.com:8080"
			parsedProxyURL, err := url.Parse(proxyAddress)
			if err != nil {
				log.Printf("Failed to parse HTTP proxy URL %s: %v", proxyAddress, err)
				http.Error(w, "Internal Server Error - Proxy Configuration", http.StatusInternalServerError)
				return
			}
			transport.Proxy = http.ProxyURL(parsedProxyURL)
		case "socks5":
			dialer, err := proxy.SOCKS5("tcp", proxyAddress, nil, proxy.Direct)
			if err != nil {
				log.Printf("Failed to create SOCKS5 dialer for %s: %v", proxyAddress, err)
				http.Error(w, "Internal Server Error - Proxy Configuration", http.StatusInternalServerError)
				return
			}
			// Check if the dialer supports DialContext
			if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
				transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
					return contextDialer.DialContext(ctx, network, addr)
				}
			} else {
				// Fallback to Dial if DialContext is not supported (though proxy.SOCKS5 should return a ContextDialer)
				dialS := dialer.Dial // Capture for the closure
				transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
					// Implement a simple context-aware dial if possible, or just call dialS
					// For simplicity here, we ignore the context if only Dial is available.
					// A more robust solution might involve selecting on ctx.Done().
					return dialS(network, addr)
				}
			}

		default:
			log.Printf("Unsupported proxy type: %s", proxyCfg.Type)
			http.Error(w, "Internal Server Error - Proxy Configuration", http.StatusInternalServerError)
			return
		}
		reverseProxy.Transport = transport
	}

	reverseProxy.Director = func(req *http.Request) {
		req.Method = r.Method
		req.Host = target.Host
		req.URL = target
		log.Printf("Forwarding request to: %s", req.URL)
		req.Header = r.Header.Clone()
		if r.Body != nil {
			req.Body = r.Body
			req.ContentLength = r.ContentLength
		}
		for name, values := range r.Header {
			req.Header.Del(name)
			for _, value := range values {
				req.Header.Add(name, value)
			}
		}
	}

	reverseProxy.ModifyResponse = func(resp *http.Response) error {
		setSecurityHeaders(resp.Header)
		return nil
	}

	reverseProxy.ServeHTTP(w, r)
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
				// Ensure rest starts with a / if it's not empty, or is empty
				potentialRest := pathname[len(prefix):]
				if len(potentialRest) > 0 && !strings.HasPrefix(potentialRest, "/") {
					// This case might happen if prefix itself doesn't end with /
					// and the path doesn't provide one immediately after.
					// Example: prefix="/api", path="/apiv1". We might want to avoid such matches
					// or handle them by ensuring `rest` starts with `/`.
					// For now, we assume prefixes are like "/serviceA/" or "/serviceB"
					// and paths are like "/serviceA/foo" or "/serviceB/bar"
					// Also ensure that if prefix ends with / and potentialRest also starts with /, one is removed.
					if strings.HasSuffix(matchedPrefix, "/") && strings.HasPrefix(potentialRest, "/") {
						matchedRest = potentialRest[1:]
					} else {
						matchedRest = potentialRest
					}
				} else {
					matchedRest = potentialRest
				}
			}
		}
	}

	return matchedPrefix, matchedRest
}
