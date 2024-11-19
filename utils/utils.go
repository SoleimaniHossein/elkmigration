package utils

import (
	"strconv"
)

// IntToString converts an int to a string
func IntToString(i int) string {
	return strconv.Itoa(i)
}

// StringToInt converts a string to an int, returns 0 and error if conversion fails
func StringToInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// StringToIntOrDefault converts a string to an int with a default fallback value if conversion fails
func StringToIntOrDefault(s string, defaultValue int) int {
	num, err := strconv.Atoi(s)
	if err != nil {
		return defaultValue
	}
	return num
}
