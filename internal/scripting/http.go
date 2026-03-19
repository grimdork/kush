package scripting

import (
	"strings"

	"github.com/d5/tengo/v2"
	"github.com/grimdork/kush/internal/httpclient"
)

// httpGetFunc is a Tengo function: http_get(url) => {status, body, headers}
func httpGetFunc(args ...tengo.Object) (tengo.Object, error) {
	if len(args) < 1 {
		return nil, tengo.ErrWrongNumArguments
	}
	url, ok := tengo.ToString(args[0])
	if !ok {
		return nil, tengo.ErrInvalidArgumentType{Name: "url", Expected: "string", Found: args[0].TypeName()}
	}

	resp, err := httpclient.Get(url, nil)
	if err != nil {
		return makeHTTPError(err.Error()), nil
	}
	return makeHTTPResult(resp), nil
}

// httpPostFunc is a Tengo function: http_post(url, body, [content_type]) => {status, body, headers}
func httpPostFunc(args ...tengo.Object) (tengo.Object, error) {
	if len(args) < 2 {
		return nil, tengo.ErrWrongNumArguments
	}
	url, ok := tengo.ToString(args[0])
	if !ok {
		return nil, tengo.ErrInvalidArgumentType{Name: "url", Expected: "string", Found: args[0].TypeName()}
	}
	body, ok := tengo.ToString(args[1])
	if !ok {
		return nil, tengo.ErrInvalidArgumentType{Name: "body", Expected: "string", Found: args[1].TypeName()}
	}

	headers := map[string]string{}
	if len(args) >= 3 {
		ct, ok := tengo.ToString(args[2])
		if ok {
			headers["Content-Type"] = ct
		}
	} else {
		// Auto-detect
		trimmed := strings.TrimSpace(body)
		if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
			(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
			headers["Content-Type"] = "application/json"
		}
	}

	resp, err := httpclient.Post(url, strings.NewReader(body), headers)
	if err != nil {
		return makeHTTPError(err.Error()), nil
	}
	return makeHTTPResult(resp), nil
}

func makeHTTPResult(resp *httpclient.Response) *tengo.Map {
	hdrs := make(map[string]tengo.Object)
	for k, vals := range resp.Headers {
		if len(vals) == 1 {
			hdrs[k] = &tengo.String{Value: vals[0]}
		} else {
			arr := make([]tengo.Object, len(vals))
			for i, v := range vals {
				arr[i] = &tengo.String{Value: v}
			}
			hdrs[k] = &tengo.Array{Value: arr}
		}
	}

	return &tengo.Map{
		Value: map[string]tengo.Object{
			"status":  &tengo.Int{Value: int64(resp.Status)},
			"body":    &tengo.String{Value: string(resp.Body)},
			"headers": &tengo.Map{Value: hdrs},
		},
	}
}

func makeHTTPError(msg string) *tengo.Map {
	return &tengo.Map{
		Value: map[string]tengo.Object{
			"status": &tengo.Int{Value: 0},
			"body":   &tengo.String{Value: ""},
			"error":  &tengo.String{Value: msg},
		},
	}
}
