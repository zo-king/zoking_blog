package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestExpectUsesConfiguredAdminOrigin(t *testing.T) {
	const expectedOrigin = "https://admin.zoking.tech"
	var receivedOrigin string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedOrigin = request.Header.Get("Origin")
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := probe{
		client:      server.Client(),
		apiURL:      server.URL,
		siteURL:     server.URL,
		adminOrigin: expectedOrigin,
	}
	if _, _, err := p.requestExpect(context.Background(), http.MethodGet, server.URL+"/api/v1/admin/auth/me", nil, "", http.StatusOK); err != nil {
		t.Fatalf("requestExpect() error = %v", err)
	}
	if receivedOrigin != expectedOrigin {
		t.Fatalf("Origin = %q, want %q", receivedOrigin, expectedOrigin)
	}
}
