package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/standard-librarian/shapeshifter"
	shapeshifterecho "github.com/standard-librarian/shapeshifter/adapters/echo"
	"github.com/standard-librarian/shapeshifter/ui"
)

func main() {
	registry := shapeshifter.NewRegistry()
	spec, err := shapeshifter.LoadSpecFile("shapeshifter.yaml", registry.Snapshot())
	if err != nil {
		log.Fatal(err)
	}
	engine, err := shapeshifter.NewEngine(spec, shapeshifter.WithObserver(loggingObserver{}))
	if err != nil {
		log.Fatal(err)
	}

	e := echo.New()
	e.Use(shapeshifterecho.Middleware(engine))
	shapeshifterecho.MountPreviewAPI(e, engine)
	mountShapeShifterUI(e)
	e.POST("/users", createUser)

	log.Fatal(e.Start(":8080"))
}

func mountShapeShifterUI(e *echo.Echo) {
	handler := http.StripPrefix("/_shapeshifter/ui", ui.Handler(
		ui.WithPreviewAPIBase("/_shapeshifter/api"),
		ui.WithTryItOut(true),
		ui.WithTryItOutBase("/"),
	))
	e.GET("/_shapeshifter/ui", func(c echo.Context) error {
		return c.Redirect(http.StatusFound, "/_shapeshifter/ui/")
	})
	e.GET("/_shapeshifter/ui/*", echo.WrapHandler(handler))
}

type loggingObserver struct{}

func (loggingObserver) OnShapeShifterEvent(event shapeshifter.Event) {
	fields := []any{
		"kind", event.Kind,
		"route", event.Route.Method + " " + event.Route.Path,
	}
	if event.ContractID != "" {
		fields = append(fields, "contract", event.ContractID)
	}
	if event.Phase != "" {
		fields = append(fields, "phase", event.Phase)
	}
	if event.Stage != "" {
		fields = append(fields, "stage", event.Stage)
	}
	if event.Reason != "" {
		fields = append(fields, "reason", event.Reason)
	}
	if event.Duration > 0 {
		fields = append(fields, "duration", event.Duration.Round(time.Microsecond))
	}
	if event.InBytes > 0 || event.OutBytes > 0 {
		fields = append(fields, "bytes", map[string]int{"in": event.InBytes, "out": event.OutBytes})
	}
	if event.Err != nil {
		fields = append(fields, "error", event.Err)
	}
	log.Println(fields...)
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
