package main

import (
  "net/http"

  "github.com/labstack/echo/v4"
)

func main() {
  e := echo.New()

  e.GET(("/", func(c echo.Context) error {
		// Get the contents of the GET request
		query := c.QueryParam("query")

		// Echo the contents of the GET request
		return c.String(http.StatusOK, query)
	})

	e.Logger.Fatal(e.Start(":8080"))

}
