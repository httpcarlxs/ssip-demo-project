package main

import (
	"io"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestEcho(t *testing.T) {
	// Start the echo server
	go func() {
		e := echo.New()

		e.GET("/", func(c echo.Context) error {
			// Get the contents of the GET request
			query := c.QueryParam("query")

			// Echo the contents of the GET request
			return c.String(http.StatusOK, query)
		})

		e.Logger.Fatal(e.Start(":8080"))
	}()

	// Make a GET request to the server
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://localhost:8080/?query=hello", nil)
	if err != nil {
		t.Errorf("Error creating request: %v", err)
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		t.Errorf("Error sending request: %v", err)
	}

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", resp.StatusCode)
	}

	// Check the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("Error reading response body: %v", err)
	}

	if string(body) != "hello" {
		t.Errorf("Expected response body 'hello', got '%s'", string(body))
	}
}