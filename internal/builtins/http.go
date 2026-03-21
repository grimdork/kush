package builtins

import (
	"fmt"
	"os"
	"strings"

	"github.com/grimdork/kush/internal/httpclient"
)

func init() {
	Register("get", handleGet)
	Register("post", handlePost)
	Register("put", handlePut)
	Register("delete", handleDelete)
	Register("head", handleHead)
	Register("fetch", handleFetch)
}

// parseHTTPArgs extracts flags from an HTTP command line.
// Supports -H "Key: Value" (repeatable) and -j (JSON pretty-print).
// Returns url, headers map, jsonPretty flag, remaining args.
func parseHTTPArgs(line string) (url string, headers map[string]string, jsonPretty bool, rest []string) {
	headers = make(map[string]string)
	tokens := shellSplit(line)
	if len(tokens) < 2 {
		return "", headers, false, nil
	}
	tokens = tokens[1:] // skip command name

	i := 0
	for i < len(tokens) {
		switch tokens[i] {
		case "-H":
			if i+1 < len(tokens) {
				i++
				parts := strings.SplitN(tokens[i], ":", 2)
				if len(parts) == 2 {
					headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
				}
			}
		case "-j":
			jsonPretty = true
		default:
			if url == "" {
				url = tokens[i]
			} else {
				rest = append(rest, tokens[i])
			}
		}
		i++
	}
	return
}

func handleGet(b *Builtins, line string) bool {
	url, headers, jsonPretty, _ := parseHTTPArgs(line)
	if url == "" {
		fmt.Fprintln(os.Stderr, "usage: get [-j] [-H \"Key: Value\"] <url>")
		return true
	}

	resp, err := httpclient.Get(url, headers)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return true
	}

	if jsonPretty || httpclient.IsJSON(resp.Headers) {
		httpclient.PrettyJSON(resp.Body)
	} else {
		os.Stdout.Write(resp.Body)
		// Ensure trailing newline
		if len(resp.Body) > 0 && resp.Body[len(resp.Body)-1] != '\n' {
			fmt.Println()
		}
	}
	return true
}

func handlePost(b *Builtins, line string) bool {
	url, headers, jsonPretty, rest := parseHTTPArgs(line)
	if url == "" {
		fmt.Fprintln(os.Stderr, "usage: post [-j] [-H \"Key: Value\"] <url> [body]")
		return true
	}

	body := strings.NewReader(strings.Join(rest, " "))
	if _, ok := headers["Content-Type"]; !ok && len(rest) > 0 {
		// Auto-detect JSON body
		joined := strings.Join(rest, " ")
		if (strings.HasPrefix(joined, "{") && strings.HasSuffix(joined, "}")) ||
			(strings.HasPrefix(joined, "[") && strings.HasSuffix(joined, "]")) {
			headers["Content-Type"] = "application/json"
		} else {
			headers["Content-Type"] = "application/x-www-form-urlencoded"
		}
	}

	resp, err := httpclient.Post(url, body, headers)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return true
	}

	if jsonPretty || httpclient.IsJSON(resp.Headers) {
		httpclient.PrettyJSON(resp.Body)
	} else {
		os.Stdout.Write(resp.Body)
		if len(resp.Body) > 0 && resp.Body[len(resp.Body)-1] != '\n' {
			fmt.Println()
		}
	}
	return true
}

func handlePut(b *Builtins, line string) bool {
	url, headers, jsonPretty, rest := parseHTTPArgs(line)
	if url == "" {
		fmt.Fprintln(os.Stderr, "usage: put [-j] [-H \"Key: Value\"] <url> [body]")
		return true
	}

	body := strings.NewReader(strings.Join(rest, " "))
	if _, ok := headers["Content-Type"]; !ok && len(rest) > 0 {
		joined := strings.Join(rest, " ")
		if (strings.HasPrefix(joined, "{") && strings.HasSuffix(joined, "}")) ||
			(strings.HasPrefix(joined, "[") && strings.HasSuffix(joined, "]")) {
			headers["Content-Type"] = "application/json"
		} else {
			headers["Content-Type"] = "application/x-www-form-urlencoded"
		}
	}

	resp, err := httpclient.Put(url, body, headers)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return true
	}

	if jsonPretty || httpclient.IsJSON(resp.Headers) {
		httpclient.PrettyJSON(resp.Body)
	} else {
		os.Stdout.Write(resp.Body)
		if len(resp.Body) > 0 && resp.Body[len(resp.Body)-1] != '\n' {
			fmt.Println()
		}
	}
	return true
}

func handleDelete(b *Builtins, line string) bool {
	url, headers, jsonPretty, _ := parseHTTPArgs(line)
	if url == "" {
		fmt.Fprintln(os.Stderr, "usage: delete [-j] [-H \"Key: Value\"] <url>")
		return true
	}

	resp, err := httpclient.Delete(url, headers)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return true
	}

	if jsonPretty || httpclient.IsJSON(resp.Headers) {
		httpclient.PrettyJSON(resp.Body)
	} else {
		os.Stdout.Write(resp.Body)
		if len(resp.Body) > 0 && resp.Body[len(resp.Body)-1] != '\n' {
			fmt.Println()
		}
	}
	return true
}

func handleHead(b *Builtins, line string) bool {
	url, headers, _, _ := parseHTTPArgs(line)
	if url == "" {
		fmt.Fprintln(os.Stderr, "usage: head [-H \"Key: Value\"] <url>")
		return true
	}

	resp, err := httpclient.Head(url, headers)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return true
	}

	fmt.Printf("HTTP %s\n", resp.StatusText)
	httpclient.PrintHeaders(resp.Headers)
	return true
}

func handleFetch(b *Builtins, line string) bool {
	tokens := shellSplit(line)
	if len(tokens) < 2 {
		fmt.Fprintln(os.Stderr, "usage: fetch [-o outfile] <url>")
		return true
	}
	tokens = tokens[1:]

	var outPath string
	var url string
	headers := make(map[string]string)

	for i := 0; i < len(tokens); i++ {
		switch tokens[i] {
		case "-o":
			if i+1 < len(tokens) {
				i++
				outPath = tokens[i]
			}
		case "-H":
			if i+1 < len(tokens) {
				i++
				parts := strings.SplitN(tokens[i], ":", 2)
				if len(parts) == 2 {
					headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
				}
			}
		default:
			if url == "" {
				url = tokens[i]
			}
		}
	}

	if url == "" {
		fmt.Fprintln(os.Stderr, "usage: fetch [-o outfile] <url>")
		return true
	}

	if err := httpclient.Download(url, outPath, headers); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	} else if outPath != "" {
		fmt.Printf("saved to %s\n", outPath)
	}
	return true
}
