//go:build !windows

package captcha

import (
	"context"
	"errors"
)

func Solve(context.Context, string, string) (string, error) {
	return "", errors.New("окно капчи доступно только в Windows")
}
