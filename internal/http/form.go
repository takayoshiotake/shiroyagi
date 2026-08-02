package httpserver

import "strconv"

func portValue(ok bool, port int) string {
	if !ok {
		return ""
	}
	return strconv.Itoa(port)
}
