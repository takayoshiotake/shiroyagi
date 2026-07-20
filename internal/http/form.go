package httpserver

import "strconv"

func selected(ok bool) string {
	if ok {
		return " selected"
	}
	return ""
}

func smtpSecuritySelected(current string, value string) string {
	if current == "" {
		current = "plain"
	}
	return selected(current == value)
}

func portValue(ok bool, port int) string {
	if !ok {
		return ""
	}
	return strconv.Itoa(port)
}
