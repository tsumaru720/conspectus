package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"conspectus/internal/httpapi"
)

const loopbackBase = "http://conspectus.internal"

type Client struct {
	base string
	rt   http.RoundTripper
}

func NewLoopback(handler http.Handler) *Client {
	return &Client{base: loopbackBase, rt: &cachingRT{base: &loopbackRT{handler: handler}}}
}

func NewRemote(base string) *Client {
	return &Client{base: strings.TrimRight(base, "/"), rt: &cachingRT{base: http.DefaultTransport}}
}

type loopbackRT struct{ handler http.Handler }

func (rt *loopbackRT) RoundTrip(req *http.Request) (*http.Response, error) {
	req = httpapi.WithInternalRequest(req)
	rec := newRecorder()
	rt.handler.ServeHTTP(rec, req)
	return rec.result(), nil
}

type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("api error (status %d)", e.Status)
}

func (c *Client) Do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("apiclient: encode body: %w", err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rdr)
	if err != nil {
		return nil, fmt.Errorf("apiclient: %s %s: %w", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	hc := &http.Client{Transport: c.rt}
	return hc.Do(req)
}

type envelope struct {
	Data json.RawMessage `json:"data"`
	Meta *struct {
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
		Total   int `json:"total"`
	} `json:"meta"`
	Error *struct {
		Code    string   `json:"code"`
		Message string   `json:"message"`
		Details []string `json:"details"`
	} `json:"error"`
}

func (c *Client) do(ctx context.Context, method, path string, body any) (json.RawMessage, *int, error) {
	resp, err := c.Do(ctx, method, path, body)
	if err != nil {
		return nil, nil, fmt.Errorf("apiclient: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("apiclient: %s %s: read body: %w", method, path, err)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, nil, fmt.Errorf("apiclient: %s %s: status %d, undecodable body: %w", method, path, resp.StatusCode, err)
	}
	if env.Error != nil || resp.StatusCode >= 400 {
		msg := ""
		code := "http_error"
		if env.Error != nil {
			msg = env.Error.Message
			code = env.Error.Code
			if len(env.Error.Details) > 0 {
				msg += " (" + strings.Join(env.Error.Details, "; ") + ")"
			}
		}
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
		}
		return nil, nil, &Error{Status: resp.StatusCode, Code: code, Message: msg}
	}
	var total *int
	if env.Meta != nil {
		t := env.Meta.Total
		total = &t
	}
	return env.Data, total, nil
}

func (c *Client) Get(ctx context.Context, path string, out any) error {
	return c.getDelete(ctx, http.MethodGet, path, out)
}

func (c *Client) Delete(ctx context.Context, path string, out any) error {
	return c.getDelete(ctx, http.MethodDelete, path, out)
}

func (c *Client) getDelete(ctx context.Context, method, path string, out any) error {
	data, _, err := c.do(ctx, method, path, nil)
	if err != nil {
		return err
	}
	return decodeInto(data, out)
}

func (c *Client) GetList(ctx context.Context, path string, out any) (total int, err error) {
	data, totalPtr, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return 0, err
	}
	if err := decodeInto(data, out); err != nil {
		return 0, err
	}
	if totalPtr != nil {
		total = *totalPtr
	}
	return total, nil
}

func (c *Client) Post(ctx context.Context, path string, body, out any) error {
	data, _, err := c.do(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	return decodeInto(data, out)
}

func (c *Client) Patch(ctx context.Context, path string, body, out any) error {
	data, _, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return err
	}
	return decodeInto(data, out)
}

func decodeInto(data json.RawMessage, out any) error {
	if out == nil || data == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("apiclient: decode data: %w", err)
	}
	return nil
}

func Query(path string, kv ...any) string {
	v := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		key := fmt.Sprint(kv[i])
		val := fmt.Sprint(kv[i+1])
		if val != "" && val != "0" && val != "<nil>" {
			v.Set(key, val)
		}
	}
	q := v.Encode()
	if q == "" {
		return path
	}
	return path + "?" + q
}

func (c *Client) Base() string { return c.base }

func (c *Client) StdClient() *http.Client { return &http.Client{Transport: c.rt} }
