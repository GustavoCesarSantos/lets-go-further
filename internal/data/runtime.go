package data

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrInvalidRuntimeFormat = errors.New("invalid runtime format")
type Runtime int32

func (r Runtime) MarshalJSON() ([]byte, error) {
    jsonValue := fmt.Sprintf("%d mins", r)
    quotedJsonValue := strconv.Quote(jsonValue)
    return []byte(quotedJsonValue), nil
}

func (r *Runtime) UnmarshalJSON(jsonValue []byte) error {
    unquotedJSONValue, err := strconv.Unquote(string(jsonValue))
    if err != nil {
        return ErrInvalidRuntimeFormat
    }
    parts := strings.Split(unquotedJSONValue, " ")
    if len(parts) != 2 || parts[1] != "mins" {
        return ErrInvalidRuntimeFormat
    }
    i, parseErr := strconv.ParseInt(parts[0], 10, 34)
    if parseErr != nil {
        return ErrInvalidRuntimeFormat
    }
    *r = Runtime(i)
    return nil
}
