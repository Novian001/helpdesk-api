package httpapi

import (
	"embed"

	"github.com/gofiber/fiber/v2"
)

//go:embed docs
var swaggerFS embed.FS

// swaggerRedirect sends the visitor to the Swagger UI page.
func swaggerRedirect(c *fiber.Ctx) error {
	return c.Redirect("/swagger/index.html", fiber.StatusMovedPermanently)
}

// serveSwagger serves the embedded Swagger UI and the OpenAPI document.
func serveSwagger(c *fiber.Ctx) error {
	path := c.Params("*")
	data, err := swaggerFS.ReadFile("docs/" + path)
	if err != nil {
		return c.SendStatus(fiber.StatusNotFound)
	}
	contentType := "text/plain; charset=utf-8"
	switch {
	case path == "swagger.json":
		contentType = "application/json"
	case path == "index.html":
		contentType = "text/html; charset=utf-8"
	}
	c.Set(fiber.HeaderContentType, contentType)
	return c.Send(data)
}
