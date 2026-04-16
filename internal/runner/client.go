package runner

import (
	"net/http"
	"strings"
	"time"
)

type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Query   map[string]string
	Body    string
	Timeout time.Duration
}

func Run(req *Request) (*http.Response, error) {
	var bodyReader *strings.Reader
	if req.Body != "" {
		bodyReader = strings.NewReader(req.Body)
	}

	var httpReq *http.Request
	var err error
	if bodyReader != nil {
		httpReq, err = http.NewRequest(req.Method, req.URL, bodyReader)
	} else {
		httpReq, err = http.NewRequest(req.Method, req.URL, nil)
	}
	if err != nil {
		return nil, err
	}

	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	if len(req.Query) > 0 {
		q := httpReq.URL.Query()
		for k, v := range req.Query {
			q.Set(k, v)
		}
		httpReq.URL.RawQuery = q.Encode()
	}

	client := &http.Client{
		Timeout: req.Timeout,
	}

	return client.Do(httpReq)
}
