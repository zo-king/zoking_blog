package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ErrorBody struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"data":       data,
		"request_id": requestID(c),
	})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, gin.H{
		"data":       data,
		"request_id": requestID(c),
	})
}

func Fail(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{
		"error": ErrorBody{
			Code:    code,
			Message: message,
		},
		"request_id": requestID(c),
	})
}

func requestID(c *gin.Context) string {
	value, ok := c.Get("request_id")
	if !ok {
		return ""
	}
	id, _ := value.(string)
	return id
}
