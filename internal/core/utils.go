package core

import (
	"regexp"
	"strings"
)

func InsertPipesInTheMiddle(input string) string {
	var output strings.Builder
	re := regexp.MustCompile(`\s+`)
	words := strings.Fields(input)
	spaces := re.FindAllString(input, -1)

	for i := 0; i < len(words); i++ {
		if i < len(spaces) {
			middle := (len(spaces[i]) / 2) - 1
			output.WriteString(words[i] + strings.Repeat(" ", middle) + "|" + strings.Repeat(" ", middle))
		} else {
			output.WriteString(words[i] + " |")
		}
	}

	return output.String()
}
