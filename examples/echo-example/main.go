package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/standard-librarian/shapeshifter"
	shapeshifterecho "github.com/standard-librarian/shapeshifter/adapters/echo"
)

func main() {
	registry := shapeshifter.NewRegistry()
	spec, err := shapeshifter.LoadSpecFile("shapeshifter.yaml", registry.Snapshot())
	if err != nil {
		log.Fatal(err)
	}
	engine, err := shapeshifter.NewEngine(spec)
	if err != nil {
		log.Fatal(err)
	}

	e := echo.New()
	e.Use(shapeshifterecho.Middleware(engine))
	shapeshifterecho.MountPreviewAPI(e, engine)
	e.POST("/users", createUser)

	log.Fatal(e.Start(":8080"))
}

func createUser(c echo.Context) error {
	var input struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(c.Request().Body).Decode(&input); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{
		"internal_id": "123",
		"name":        input.Name,
		"email":       input.Email,
	})
}
