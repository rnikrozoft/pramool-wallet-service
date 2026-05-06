package omisehttp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Do sends the request with Omise basic auth, reads the body, and returns an error if status is not 2xx.
// errLabel is prefixed on error messages (e.g. "omise charge failed").
func Do(ctx context.Context, client *http.Client, secretKey string, req *http.Request, errLabel string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if client == nil {
		client = http.DefaultClient
	}
	req = req.WithContext(ctx)
	req.SetBasicAuth(strings.TrimSpace(secretKey), "")

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: status=%d body=%s", errLabel, res.StatusCode, string(body))
	}
	return body, nil
}
