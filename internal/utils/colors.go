package utils

import "fmt"

func Colorize(color string, text string) string {
	return fmt.Sprintf("\033[%sm%s\033[0m", color, text)
}
