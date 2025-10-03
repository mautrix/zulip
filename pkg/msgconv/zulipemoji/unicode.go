package zulipemoji

import (
	"strconv"
	"strings"
)

func UnifiedToUnicode(input string) string {
	input = strings.TrimPrefix(input, "emoji-")
	parts := strings.Split(input, "-")
	output := make([]rune, len(parts))
	for i, part := range parts {
		val, err := strconv.ParseInt(part, 16, 32)
		if err != nil {
			return ""
		}
		output[i] = rune(val)
	}
	return string(output)
}
