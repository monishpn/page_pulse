package models

type CustomError struct {
	Message string
	Code    int
}

func (c CustomError) Error() string   { return c.Message }
func (c CustomError) StatusCode() int { return c.Code }
