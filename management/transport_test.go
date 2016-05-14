package management

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/jmoiron/sqlx"
	"github.com/micromdm/dep"
	"github.com/micromdm/micromdm/device"
	"github.com/micromdm/micromdm/profile"
	"golang.org/x/net/context"
)

func TestShowProfile(t *testing.T) {
	server, svc := newServer(t)
	defer teardown()
	defer server.Close()

	profileData := []byte(`{
    "payload_identifier": "com.micromdm.example2",
    "data" : "fooProfile"
	}`)

	testGetHTTP(t, svc, server, "foo", http.StatusBadRequest)
	testGetHTTP(t, svc, server, "036d339c-4fe4-4d6e-a051-65fafbec8c93", http.StatusNotFound)

	testAddHTTP(t, svc, server, profileData, http.StatusCreated)
	profiles := testListHTTP(t, svc, server, http.StatusOK)
	for _, p := range profiles {
		returned := testGetHTTP(t, svc, server, p.UUID, http.StatusOK)
		// check that we get what we pass in
		if returned.PayloadIdentifier != p.PayloadIdentifier {
			t.Fatal("expected", p.PayloadIdentifier, "got", returned.PayloadIdentifier)
		}
	}

}

func testGetHTTP(t *testing.T, svc Service, server *httptest.Server, uuid string, expectedStatus int) *profile.Profile {
	client := http.DefaultClient
	theURL := server.URL + "/management/v1/profiles" + "/" + uuid
	resp, err := client.Get(theURL)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != expectedStatus {
		io.Copy(os.Stdout, resp.Body)
		t.Fatal("expected", expectedStatus, "got", resp.StatusCode)
	}

	// test decoding the result into a struct
	var profile profile.Profile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		t.Log("failed to decode profiles from GET response")
		t.Fatal(err)
	}
	return &profile
}

func TestListProfiles(t *testing.T) {
	server, svc := newServer(t)
	defer teardown()
	defer server.Close()

	profileData := []byte(`{
    "payload_identifier": "com.micromdm.example2",
    "data" : "fooProfile"
	}`)

	testAddHTTP(t, svc, server, profileData, http.StatusCreated)
	testListHTTP(t, svc, server, http.StatusOK)

}

func TestAddProfile(t *testing.T) {
	server, svc := newServer(t)
	defer teardown()
	defer server.Close()

	profileData := []byte(`{
    "payload_identifier": "com.micromdm.example2",
    "data" : "fooProfile"
	}`)

	var addTests = []struct {
		in       []byte
		expected int
	}{
		{
			in:       nil,
			expected: http.StatusBadRequest,
		},
		{
			in:       profileData,
			expected: http.StatusCreated,
		},
		{
			in:       profileData,
			expected: http.StatusConflict,
		},
	}

	for _, tt := range addTests {
		testAddHTTP(t, svc, server, tt.in, tt.expected)
	}
}

var testConn = "user=micromdm password=micromdm dbname=micromdm sslmode=disable"

func newServer(t *testing.T) (*httptest.Server, Service) {
	ctx := context.Background()
	l := log.NewLogfmtLogger(os.Stderr)
	logger := log.NewContext(l).With("source", "testing")
	ds, err := device.NewDB("postgres", testConn, logger)
	if err != nil {
		t.Fatal(err)
	}
	ps, err := profile.NewDB("postgres", testConn, logger)
	if err != nil {
		t.Fatal(err)
	}

	svc := NewService(ds, ps, nil)
	handler := ServiceHandler(ctx, svc, logger)
	server := httptest.NewServer(handler)
	return server, svc
}

func testListHTTP(t *testing.T, svc Service, server *httptest.Server, expectedStatus int) []profile.Profile {
	client := http.DefaultClient
	theURL := server.URL + "/management/v1/profiles"
	resp, err := client.Get(theURL)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != expectedStatus {
		io.Copy(os.Stdout, resp.Body)
		t.Fatal("expected", expectedStatus, "got", resp.StatusCode)
	}

	// test decoding the result into a struct
	var profiles []profile.Profile
	if err := json.NewDecoder(resp.Body).Decode(&profiles); err != nil {
		t.Log("failed to decode profiles from list response")
		t.Fatal(err)
	}
	return profiles
}

func testAddHTTP(t *testing.T, svc Service, server *httptest.Server, profile []byte, expectedStatus int) {
	body := &nopCloser{bytes.NewBuffer(profile)}

	client := http.DefaultClient
	theURL := server.URL + "/management/v1/profiles"
	resp, err := client.Post(theURL, "application/json", body)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != expectedStatus {
		io.Copy(os.Stdout, resp.Body)
		t.Fatal("expected", expectedStatus, "got", resp.StatusCode)
	}
}

func TestFetchDEPDevices(t *testing.T) {
	ctx := context.Background()
	logger := log.NewLogfmtLogger(os.Stderr)
	ds, err := device.NewDB("postgres", testConn, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	// Auth configuration
	config := &dep.Config{
		ConsumerKey:    "CK_48dd68d198350f51258e885ce9a5c37ab7f98543c4a697323d75682a6c10a32501cb247e3db08105db868f73f2c972bdb6ae77112aea803b9219eb52689d42e6",
		ConsumerSecret: "CS_34c7b2b531a600d99a0e4edcf4a78ded79b86ef318118c2f5bcfee1b011108c32d5302df801adbe29d446eb78f02b13144e323eb9aad51c79f01e50cb45c3a68",
		AccessToken:    "AT_927696831c59ba510cfe4ec1a69e5267c19881257d4bca2906a99d0785b785a6f6fdeb09774954fdd5e2d0ad952e3af52c6d8d2f21c924ba0caf4a031c158b89",
		AccessSecret:   "AS_c31afd7a09691d83548489336e8ff1cb11b82b6bca13f793344496a556b1f4972eaff4dde6deb5ac9cf076fdfa97ec97699c34d515947b9cf9ed31c99dded6ba",
	}

	dc, err := dep.NewClient(config, dep.ServerURL("http://localhost:9000"))
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(ds, nil, dc)
	handler := ServiceHandler(ctx, svc, logger)
	server := httptest.NewServer(handler)
	defer server.Close()

	client := http.DefaultClient
	theURL := server.URL + "/management/v1/devices/fetch"
	resp, err := client.Post(theURL, "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		io.Copy(os.Stdout, resp.Body)
		t.Fatal("expected", http.StatusOK, "got", resp.StatusCode)
	}
}

// a face io.ReadCloser for constructing request Body
type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func teardown() {
	db, err := sqlx.Open("postgres", testConn)
	if err != nil {
		panic(err)
	}

	drop := `
	DROP TABLE IF EXISTS device_workflow;
	DROP TABLE IF EXISTS devices;
	DROP INDEX IF EXISTS devices.serial_idx;
	DROP INDEX IF EXISTS devices.udid_idx;
	DROP TABLE IF EXISTS profiles;
	`
	db.MustExec(drop)
	defer db.Close()
}