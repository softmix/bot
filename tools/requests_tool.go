package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func init() {
	RegisterTool(HTTPGetTool{})
	RegisterTool(HTTPPostTool{})
	RegisterTool(HTTPPatchTool{})
	RegisterTool(HTTPPutTool{})
	RegisterTool(HTTPDeleteTool{})
	RegisterTool(HTTPHeadTool{})
}

type HTTPGetTool struct{}
type HTTPPostTool struct{}
type HTTPPatchTool struct{}
type HTTPPutTool struct{}
type HTTPDeleteTool struct{}
type HTTPHeadTool struct{}

func (tool HTTPGetTool) Name() string    { return "http_get" }
func (tool HTTPPostTool) Name() string   { return "http_post" }
func (tool HTTPPatchTool) Name() string  { return "http_patch" }
func (tool HTTPPutTool) Name() string    { return "http_put" }
func (tool HTTPDeleteTool) Name() string { return "http_delete" }
func (tool HTTPHeadTool) Name() string   { return "http_head" }

func (tool HTTPGetTool) Description() string {
	return "A portal to the internet. Use this when you need to get specific content from a website. Input should be a url (i.e. https://www.google.com). The output will be the text response of the GET request."
}
func (tool HTTPDeleteTool) Description() string {
	return "A portal to the internet. Use this when you need to make a DELETE request to a URL. Input should be a specific url, and the output will be the text response of the DELETE request."
}
func (tool HTTPHeadTool) Description() string {
	return "A portal to the internet. Use this when you need to make a HEAD request to a URL. Input should be a specific url, and the output will be the text response of the HEAD request."
}
func (tool HTTPPostTool) Description() string {
	return "Use this when you want to POST to a website. Input should be a json string with two keys: \"url\" and \"data\". The value of \"url\" should be a string, and the value of \"data\" should be a dictionary of key-value pairs you want to POST to the url. Be careful to always use double quotes for strings in the json string The output will be the text response of the POST request."
}
func (tool HTTPPatchTool) Description() string {
	return "Use this when you want to PATCH to a website. Input should be a json string with two keys: \"url\" and \"data\". The value of \"url\" should be a string, and the value of \"data\" should be a dictionary of key-value pairs you want to PATCH to the url. Be careful to always use double quotes for strings in the json string The output will be the text response of the PATCH request."
}
func (tool HTTPPutTool) Description() string {
	return "Use this when you want to PUTS to a website. Input should be a json string with two keys: \"url\" and \"data\". The value of \"url\" should be a string, and the value of \"data\" should be a dictionary of key-value pairs you want to PUT to the url. Be careful to always use double quotes for strings in the json string The output will be the text response of the PUT request."
}

func (tool HTTPGetTool) Run(input string) (string, error)    { return requestHelper("GET", input) }
func (tool HTTPPostTool) Run(input string) (string, error)   { return requestHelper("POST", input) }
func (tool HTTPPatchTool) Run(input string) (string, error)  { return requestHelper("PATCH", input) }
func (tool HTTPPutTool) Run(input string) (string, error)    { return requestHelper("PUT", input) }
func (tool HTTPDeleteTool) Run(input string) (string, error) { return requestHelper("DELETE", input) }
func (tool HTTPHeadTool) Run(input string) (string, error)   { return requestHelper("HEAD", input) }

func requestHelper(method, input string) (string, error) {
	var args struct {
		URL  string `json:"url"`
		Data string `json:"data"`
	}

	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("error parsing JSON arguments: %v", err)
	}

	client := &http.Client{}
	var reqBody io.Reader
	if args.Data != "" {
		reqBody = strings.NewReader(args.Data)
	}

	req, err := http.NewRequest(method, args.URL, reqBody)
	if err != nil {
		return "", err
	}

	if args.Data != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
