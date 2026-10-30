package colorkwingt

import (
	"barbtils/internal/logger"
	"strings"

	ck "github.com/washingmachine55/color-kwing/api"
)

func GetColorNameFromHexCode(input string) string {
	if !strings.Contains(input, "#") {
		input = "#" + input
	}

	hex, err := ck.GetColorName(input)
	if err != nil {
		logger.Fatalf("err: %v\n", err)
	}
	return hex
}
