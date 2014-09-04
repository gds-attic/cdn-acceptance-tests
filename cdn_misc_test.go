package main

import (
	"io/ioutil"
	"testing"

	"./fake_http"
)

// Should redirect from HTTP to HTTPS without hitting origin, whilst
// preserving path and query params.
func TestMiscProtocolRedirect(t *testing.T) {
	ResetBackends(backendsByPriority)

	const reqPath = "/one/two"
	const reqProto = "http"
	const expectedProto = "https"
	const expectedStatus = fake_http.StatusMovedPermanently
	const headerName = "Location"
	var expectedURL string

	originServer.SwitchHandler(func(w fake_http.ResponseWriter, r *fake_http.Request) {
		t.Error("Request should not have made it to origin")
	})

	req := NewUniqueEdgeGET(t)
	req.URL.Path = reqPath
	req.URL.Scheme = reqProto

	if len(req.URL.RawQuery) == 0 {
		t.Fatal("Request must have query params to test preservation")
	}

	resp := RoundTripCheckError(t, req)
	defer resp.Body.Close()

	req.URL.Scheme = expectedProto
	expectedURL = req.URL.String()

	if resp.StatusCode != expectedStatus {
		t.Errorf(
			"Received incorrect status code. Expected %d, got %d",
			expectedStatus,
			resp.StatusCode,
		)
	}
	if dest := resp.Header.Get(headerName); dest != expectedURL {
		t.Errorf(
			"Received incorrect %q header. Expected %q, got %q",
			expectedURL,
			dest,
		)
	}
}

// Should return 403 and not invalidate the edge's cache for PURGE requests
// that come from IPs not in the whitelist. We assume that this is not
// running from a whitelisted address.
func TestMiscRestrictPurgeRequests(t *testing.T) {
	ResetBackends(backendsByPriority)

	var expectedBody string
	var expectedStatus int
	req := NewUniqueEdgeGET(t)

	for requestCount := 1; requestCount < 4; requestCount++ {
		switch requestCount {
		case 1:
			req.Method = "GET"
			expectedBody = "this should not be purged"
			expectedStatus = 200

			originServer.SwitchHandler(func(w fake_http.ResponseWriter, r *fake_http.Request) {
				w.Write([]byte(expectedBody))
			})
		case 2:
			req.Method = "PURGE"
			expectedBody = ""
			expectedStatus = 403

			originServer.SwitchHandler(func(w fake_http.ResponseWriter, r *fake_http.Request) {
				t.Error("Request should not have made it to origin")
				w.Write([]byte(originServer.Name))
			})
		case 3:
			req.Method = "GET"
			expectedBody = "this should not be purged"
			expectedStatus = 200
		}

		resp := RoundTripCheckError(t, req)
		defer resp.Body.Close()

		if resp.StatusCode != expectedStatus {
			t.Errorf(
				"Request %d received incorrect status code. Expected %d, got %d",
				requestCount,
				expectedStatus,
				resp.StatusCode,
			)
		}

		if expectedBody != "" {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}

			if bodyStr := string(body); bodyStr != expectedBody {
				t.Errorf(
					"Request %d received incorrect response body. Expected %q, got %q",
					requestCount,
					expectedBody,
					bodyStr,
				)
			}
		}
	}
}
