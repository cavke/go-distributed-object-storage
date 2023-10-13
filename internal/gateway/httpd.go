package gateway

import (
	"fmt"
	"github.com/cavke/go-distributed-object-storage/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"io"
	"log"
	"net/http"
	"regexp"
)

var alphanumericRegex = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

func NewServer(s storage.Storage) *echo.Echo {
	// echo instance
	e := echo.New()

	// middlewares
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// routes
	e.GET("/object/:id", func(c echo.Context) error { return getObject(s, c) })
	e.PUT("/object/:id", func(c echo.Context) error { return putObject(s, c) })

	return e
}

type Response struct {
	Message string `json:"message"`
}

func validateObjectID(id string) bool {
	if len(id) == 0 || len(id) > 32 {
		return false
	}
	return alphanumericRegex.MatchString(id)
}

func getObject(s storage.Storage, c echo.Context) error {
	ctx := c.Request().Context()
	objectID := c.Param("id")

	if !validateObjectID(objectID) {
		return c.JSON(http.StatusBadRequest, Response{Message: "Invalid objectID. Must be alfa-numeric string between 1 and 32 characters."})
	}

	// retrieve object from storage
	object, err := s.Get(ctx, objectID)
	if err != nil {
		log.Printf("Cannot retrieve object: %v", err)
		return c.JSON(http.StatusInternalServerError, Response{Message: fmt.Sprintf("Error retrieving object: %s", objectID)})
	}
	if object == nil {
		return c.JSON(http.StatusNotFound, Response{Message: fmt.Sprintf("Object doesn't exist: %s", objectID)})
	}

	return c.Blob(http.StatusOK, object.ContentType, object.Content)
}

func putObject(s storage.Storage, c echo.Context) error {
	ctx := c.Request().Context()
	contentType := c.Request().Header.Get(echo.HeaderContentType)
	objectID := c.Param("id")

	if !validateObjectID(objectID) {
		return c.JSON(http.StatusBadRequest, Response{Message: "Invalid objectID. Must be alfa-numeric string between 1 and 32 characters."})
	}

	// read object bytes from request body
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		log.Printf("Cannot read request body: %v", err)
		return c.JSON(http.StatusBadRequest, Response{Message: "Cannot read request body"})
	}

	// put object to storage
	object := storage.Object{
		ID:          objectID,
		ContentType: contentType,
		Content:     body,
	}
	err = s.Put(ctx, &object)
	if err != nil {
		log.Printf("Cannot store object: %v", err)
		return c.JSON(http.StatusInternalServerError, Response{Message: fmt.Sprintf("Cannot store object: %s", objectID)})
	}

	return c.JSON(http.StatusOK, Response{Message: fmt.Sprintf("Object was successfully stored with ID: %s", objectID)})
}
