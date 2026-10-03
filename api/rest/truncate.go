package rest

// truncateBody returns at most n bytes of body, for quoting a request body
// in logs and errors.
func truncateBody(body []byte, n int) []byte {
	if len(body) < n {
		return body
	}
	return body[:n+1]
}
