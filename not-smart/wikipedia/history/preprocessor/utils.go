package preprocessor

import "net/url"

// Prepare URLs'
func (s *Preprocessor) prepareRootURLs() (string, error) {
	u, err := url.Parse(s.ROOT_URL)
	if err != nil {
		return "", err
	}

	baseQueries := map[string]string{
		"action":        "query",
		"format":        "json",
		"formatversion": "2",
	}
	params := url.Values{}
	for key, val := range baseQueries {
		params.Set(key, val)
	}
	u.RawQuery = params.Encode()

	return u.String(), nil
}
func (s *Preprocessor) prepareUserURL() (*url.URL, error) {
	u, err := url.Parse(s.BASE_URL)
	if err != nil {
		return nil, err
	}

	pageQueries := map[string]string{
		"list":      "users",
		"usprop":    "groups|editcount|registration",
		"ususerids": "",
	}
	params := u.Query()
	for key, val := range pageQueries {
		params.Set(key, val)
	}
	u.RawQuery = params.Encode()

	return u, nil
}
