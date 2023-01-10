package infra

import (
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
)

type Response struct {
	Status  int         `json:"status"`
	Code    string      `json:"code"`
	Message interface{} `json:"message"`
	Ack     int64       `json:"ack"`
}

func Result(status int, msg interface{}, c *fiber.Ctx) error {
	return c.Status(status).JSON(Response{
		Code:    http.StatusText(status),
		Status:  status,
		Message: msg,
		Ack:     time.Now().UnixMilli(),
	})
}

func Ok(c *fiber.Ctx) error {
	return Result(http.StatusOK, "success", c)
}

func OkWithMessage(message interface{}, c *fiber.Ctx) error {
	return Result(http.StatusOK, message, c)
}

func OkWithRaw(contentType string, data []byte, c *fiber.Ctx) error {
	return c.Status(200).Type(contentType).Send(data)
}

func FailWithEmptyRaw(contentType string, c *fiber.Ctx) error {
	return c.Status(204).Type(contentType).Send(nil)
}

func FailWithNotFound(contentType string, c *fiber.Ctx) error {
	return c.Status(404).Type(contentType).Send(nil)
}

func Fail(status int, c *fiber.Ctx) error {
	return Result(status, http.StatusText(status), c)
}

func FailWithMessage(status int, message interface{}, c *fiber.Ctx) error {
	return Result(status, message, c)
}
