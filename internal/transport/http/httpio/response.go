package httpio

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	status int
	body   any
}
type Handler func(*http.Request) (*Response, error)

func JSON(status int, body any) *Response { return &Response{status, body} }
func Wrap(h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := h(r)
		if err != nil || res == nil {
			res = JSON(500, map[string]string{"error": "internal error", "code": "internal_error"})
		}
		b, err := json.Marshal(res.body)
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(res.status)
		if _, err = w.Write(append(b, '\n')); err != nil {
			if observer, ok := r.Context().Value(writeObserverKey{}).(func(error)); ok {
				observer(err)
			}
		}
	}
}

type writeObserverKey struct{}
