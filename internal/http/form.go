package httpserver

import "strconv"

func selected(ok bool) string {
	if ok {
		return " selected"
	}
	return ""
}

func portValue(ok bool, port int) string {
	if !ok {
		return ""
	}
	return strconv.Itoa(port)
}
