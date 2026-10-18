package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)


func TestFindUsers_ValidationErrors(t *testing.T) {
	client := &SearchClient{
		AccessToken: ValidToken,
		URL:         "http://dummy.url",
	}

	tests := []struct {
		name        string
		req         SearchRequest
		expectedErr string
	}{
		{
			name:        "Negative limit",
			req:         SearchRequest{Limit: -1},
			expectedErr: "limit must be > 0",
		},
		{
			name:        "Negative offset",
			req:         SearchRequest{Offset: -1},
			expectedErr: "offset must be > 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.FindUsers(tt.req)
			if resp != nil {
				t.Errorf("expected nil response, got %+v", resp)
			}
			if err == nil || !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("expected error containing %q, got %v", tt.expectedErr, err)
			}
		})
	}
}


func TestFindUsers_NetworkAndClientErrors(t *testing.T) {
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	slowClient := &SearchClient{
		AccessToken: ValidToken,
		URL:         slowServer.URL,
	}
	_, err := slowClient.FindUsers(SearchRequest{Limit: 1})
	if err == nil || !strings.Contains(err.Error(), "timeout for") {
		t.Errorf("expected timeout error, got %v", err)
	}


	closedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := closedServer.URL
	closedServer.Close()

	brokenClient := &SearchClient{
		AccessToken: ValidToken,
		URL:         closedURL,
	}
	_, err = brokenClient.FindUsers(SearchRequest{Limit: 1})
	if err == nil || !strings.Contains(err.Error(), "unknown error") {
		t.Errorf("expected unknown network error, got %v", err)
	}


	badJSONServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer badJSONServer.Close()

	badJSONClient := &SearchClient{
		AccessToken: ValidToken,
		URL:         badJSONServer.URL,
	}
	_, err = badJSONClient.FindUsers(SearchRequest{Limit: 1})
	if err == nil || !strings.Contains(err.Error(), "cant unpack error json") {
		t.Errorf("expected 'cant unpack error json' error, got %v", err)
	}


	corruptResultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{invalid-json-body}"))
	}))
	defer corruptResultServer.Close()

	corruptClient := &SearchClient{
		AccessToken: ValidToken,
		URL:         corruptResultServer.URL,
	}
	_, err = corruptClient.FindUsers(SearchRequest{Limit: 1})
	if err == nil || !strings.Contains(err.Error(), "cant unpack result json") {
		t.Errorf("expected 'cant unpack result json' error, got %v", err)
	}
}


func TestSearchServer_AuthAndFileErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	client := &SearchClient{
		AccessToken: "wrong_token",
		URL:         ts.URL,
	}
	_, err := client.FindUsers(SearchRequest{Limit: 1})
	if err == nil || !strings.Contains(err.Error(), "bad AccessToken") {
		t.Errorf("expected 'bad AccessToken', got %v", err)
	}


	origDataset := DatasetPath
	defer func() { DatasetPath = origDataset }()

	client.AccessToken = ValidToken
	DatasetPath = "non_existent_file.xml"
	_, err = client.FindUsers(SearchRequest{Limit: 1})
	if err == nil || !strings.Contains(err.Error(), "SearchServer fatal error") {
		t.Errorf("expected 'SearchServer fatal error' for missing file, got %v", err)
	}


	tempDir := t.TempDir()
	corruptedPath := filepath.Join(tempDir, "corrupted.xml")
	if err := os.WriteFile(corruptedPath, []byte("<root><row><unclosed_tag>"), 0644); err != nil {
		t.Fatalf("failed to create corrupted xml: %v", err)
	}
	DatasetPath = corruptedPath
	_, err = client.FindUsers(SearchRequest{Limit: 1})
	if err == nil || !strings.Contains(err.Error(), "SearchServer fatal error") {
		t.Errorf("expected 'SearchServer fatal error' for corrupted XML, got %v", err)
	}
}

func TestSearchServer_InvalidParameters(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	client := &SearchClient{
		AccessToken: ValidToken,
		URL:         ts.URL,
	}

	_, err := client.FindUsers(SearchRequest{
		Limit:      5,
		OrderField: "invalid_field_name",
		OrderBy:    OrderByAsc,
	})
	if err == nil || !strings.Contains(err.Error(), "OrderFeld invalid_field_name invalid") {
		t.Errorf("expected 'OrderFeld invalid_field_name invalid', got %v", err)
	}

	_, err = client.FindUsers(SearchRequest{
		Limit:      5,
		OrderField: "Id",
		OrderBy:    99,
	})
	if err == nil || !strings.Contains(err.Error(), "unknown bad request error: bad order_by") {
		t.Errorf("expected 'unknown bad request error: bad order_by', got %v", err)
	}
}

func TestSearchServer_Sorting(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	client := &SearchClient{
		AccessToken: ValidToken,
		URL:         ts.URL,
	}

	tests := []struct {
		name       string
		orderField string
		orderBy    int
		check      func(t *testing.T, users []User)
	}{
		{
			name:       "Sort by Id Asc",
			orderField: "Id",
			orderBy:    OrderByAsc,
			check: func(t *testing.T, users []User) {
				if len(users) < 2 {
					t.Fatalf("too few users: %d", len(users))
				}
				if users[0].ID != 0 {
					t.Errorf("expected first user ID 0, got %d", users[0].ID)
				}
				for i := 1; i < len(users); i++ {
					if users[i-1].ID >= users[i].ID {
						t.Errorf("not sorted by ID asc: users[%d].ID=%d >= users[%d].ID=%d",
							i-1, users[i-1].ID, i, users[i].ID)
					}
				}
			},
		},
		{
			name:       "Sort by Id Desc",
			orderField: "Id",
			orderBy:    OrderByDesc,
			check: func(t *testing.T, users []User) {
				if len(users) < 2 {
					t.Fatalf("too few users: %d", len(users))
				}
				if users[0].ID != 34 {
					t.Errorf("expected first user ID 34, got %d", users[0].ID)
				}
				for i := 1; i < len(users); i++ {
					if users[i-1].ID <= users[i].ID {
						t.Errorf("not sorted by ID desc: users[%d].ID=%d <= users[%d].ID=%d",
							i-1, users[i-1].ID, i, users[i].ID)
					}
				}
			},
		},
		{
			name:       "Sort by Age Asc",
			orderField: "Age",
			orderBy:    OrderByAsc,
			check: func(t *testing.T, users []User) {
				if len(users) < 2 {
					t.Fatalf("too few users: %d", len(users))
				}
				if users[0].Age != 21 {
					t.Errorf("expected lowest age 21, got %d", users[0].Age)
				}
				for i := 1; i < len(users); i++ {
					if users[i-1].Age > users[i].Age {
						t.Errorf("not sorted by Age asc: users[%d].Age=%d > users[%d].Age=%d",
							i-1, users[i-1].Age, i, users[i].Age)
					}
				}
			},
		},
		{
			name:       "Sort by Age Desc",
			orderField: "Age",
			orderBy:    OrderByDesc,
			check: func(t *testing.T, users []User) {
				if len(users) < 2 {
					t.Fatalf("too few users: %d", len(users))
				}
				if users[0].Age != 40 {
					t.Errorf("expected highest age 40, got %d", users[0].Age)
				}
				for i := 1; i < len(users); i++ {
					if users[i-1].Age < users[i].Age {
						t.Errorf("not sorted by Age desc: users[%d].Age=%d < users[%d].Age=%d",
							i-1, users[i-1].Age, i, users[i].Age)
					}
				}
			},
		},
		{
			name:       "Sort by Name Asc",
			orderField: "Name",
			orderBy:    OrderByAsc,
			check: func(t *testing.T, users []User) {
				if len(users) < 2 {
					t.Fatalf("too few users: %d", len(users))
				}
				if users[0].Name != "Allison Valdez" {
					t.Errorf("expected first user Allison Valdez, got %s", users[0].Name)
				}
				for i := 1; i < len(users); i++ {
					if users[i-1].Name > users[i].Name {
						t.Errorf("not sorted by Name asc: users[%d].Name=%s > users[%d].Name=%s",
							i-1, users[i-1].Name, i, users[i].Name)
					}
				}
			},
		},
		{
			name:       "Sort by Name Desc",
			orderField: "Name",
			orderBy:    OrderByDesc,
			check: func(t *testing.T, users []User) {
				if len(users) < 2 {
					t.Fatalf("too few users: %d", len(users))
				}
				if users[0].Name != "Whitley Davidson" {
					t.Errorf("expected first user Whitley Davidson, got %s", users[0].Name)
				}
				for i := 1; i < len(users); i++ {
					if users[i-1].Name < users[i].Name {
						t.Errorf("not sorted by Name desc: users[%d].Name=%s < users[%d].Name=%s",
							i-1, users[i-1].Name, i, users[i].Name)
					}
				}
			},
		},
		{
			name:       "Sort by Name Default (empty OrderField) Asc",
			orderField: "",
			orderBy:    OrderByAsc,
			check: func(t *testing.T, users []User) {
				if len(users) < 2 {
					t.Fatalf("too few users: %d", len(users))
				}
				if users[0].Name != "Allison Valdez" {
					t.Errorf("expected first user Allison Valdez, got %s", users[0].Name)
				}
			},
		},
		{
			name:       "Sort AsIs (OrderByAsIs)",
			orderField: "Id",
			orderBy:    OrderByAsIs,
			check: func(t *testing.T, users []User) {
				if len(users) < 2 {
					t.Fatalf("too few users: %d", len(users))
				}
				// Original order from XML has ID 0, then 1, etc.
				if users[0].ID != 0 || users[1].ID != 1 {
					t.Errorf("expected original order (ID 0, then 1), got %d, %d", users[0].ID, users[1].ID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.FindUsers(SearchRequest{
				Limit:      25,
				OrderField: tt.orderField,
				OrderBy:    tt.orderBy,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, resp.Users)
		})
	}
}

func TestSearchServer_Query(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	client := &SearchClient{
		AccessToken: ValidToken,
		URL:         ts.URL,
	}

	resp, err := client.FindUsers(SearchRequest{
		Limit:      1,
		Offset:     0,
		Query:      "on",
		OrderField: "age",
		OrderBy:    OrderByDesc,
	})
	if err != nil {
		t.Fatalf("hw3 example failed: %v", err)
	}
	if len(resp.Users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(resp.Users))
	}
	expectedUser := User{
		ID:     13,
		Name:   "Whitley Davidson",
		Age:    40,
		Gender: "female",
	}
	if resp.Users[0].ID != expectedUser.ID || resp.Users[0].Name != expectedUser.Name || resp.Users[0].Age != expectedUser.Age {
		t.Errorf("expected user %+v, got %+v", expectedUser, resp.Users[0])
	}
	if !resp.NextPage {
		t.Errorf("expected NextPage=true since there are multiple matching users")
	}

	// 2. Query matching Name
	resp, err = client.FindUsers(SearchRequest{
		Limit: 5,
		Query: "Boyd",
	})
	if err != nil {
		t.Fatalf("query by name error: %v", err)
	}
	if len(resp.Users) != 1 || resp.Users[0].Name != "Boyd Wolf" {
		t.Errorf("expected Boyd Wolf, got %+v", resp.Users)
	}

	// 3. Query matching About only
	resp, err = client.FindUsers(SearchRequest{
		Limit: 5,
		Query: "Nulla cillum enim",
	})
	if err != nil {
		t.Fatalf("query by about error: %v", err)
	}
	if len(resp.Users) != 1 || resp.Users[0].Name != "Boyd Wolf" {
		t.Errorf("expected Boyd Wolf by About, got %+v", resp.Users)
	}

	// 4. Query matching nothing
	resp, err = client.FindUsers(SearchRequest{
		Limit: 5,
		Query: "StringThatDefinitelyDoesNotExistInXML12345",
	})
	if err != nil {
		t.Fatalf("non-matching query error: %v", err)
	}
	if len(resp.Users) != 0 {
		t.Errorf("expected 0 users for non-matching query, got %d", len(resp.Users))
	}
	if resp.NextPage {
		t.Errorf("expected NextPage=false for empty result")
	}
}


func TestSearchServer_Pagination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	client := &SearchClient{
		AccessToken: ValidToken,
		URL:         ts.URL,
	}


	resp, err := client.FindUsers(SearchRequest{
		Limit:      30,
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Users) != 25 {
		t.Errorf("expected 25 users capped, got %d", len(resp.Users))
	}
	if !resp.NextPage {
		t.Errorf("expected NextPage=true for 25 out of 35 users")
	}


	var allRetrieved []User
	offset := 0
	limit := 10
	page := 1

	for {
		resp, err := client.FindUsers(SearchRequest{
			Limit:      limit,
			Offset:     offset,
			OrderField: "Id",
			OrderBy:    OrderByAsc,
		})
		if err != nil {
			t.Fatalf("page %d error: %v", page, err)
		}
		allRetrieved = append(allRetrieved, resp.Users...)

		if page < 4 {
			if !resp.NextPage {
				t.Errorf("page %d: expected NextPage=true", page)
			}
			if len(resp.Users) != 10 {
				t.Errorf("page %d: expected 10 users, got %d", page, len(resp.Users))
			}
		} else if page == 4 {
			if resp.NextPage {
				t.Errorf("page 4: expected NextPage=false for last page")
			}
			if len(resp.Users) != 5 {
				t.Errorf("page %d: expected 5 users, got %d", page, len(resp.Users))
			}
		}

		if !resp.NextPage {
			break
		}
		offset += limit
		page++
	}

	if len(allRetrieved) != 35 {
		t.Fatalf("expected total 35 users retrieved across pages, got %d", len(allRetrieved))
	}
	for i, u := range allRetrieved {
		if u.ID != i {
			t.Errorf("user index %d has unexpected ID %d", i, u.ID)
		}
	}


	resp, err = client.FindUsers(SearchRequest{
		Limit:  10,
		Offset: 100,
	})
	if err != nil {
		t.Fatalf("offset beyond count error: %v", err)
	}
	if len(resp.Users) != 0 {
		t.Errorf("expected 0 users for offset beyond count, got %d", len(resp.Users))
	}
	if resp.NextPage {
		t.Errorf("expected NextPage=false for offset beyond count")
	}
}

func TestSearchServer_DirectRequests(t *testing.T) {

	req := httptest.NewRequest("GET", "/?order_by=not_a_number", nil)
	req.Header.Set("AccessToken", ValidToken)
	w := httptest.NewRecorder()
	SearchServer(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for non-integer order_by, got %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/?offset=-5&limit=-10", nil)
	req.Header.Set("AccessToken", ValidToken)
	w = httptest.NewRecorder()
	SearchServer(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for negative offset/limit, got %d", w.Code)
	}
	var users []User
	if err := json.Unmarshal(w.Body.Bytes(), &users); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(users) != 35 {
		t.Errorf("expected 35 users returned, got %d", len(users))
	}

	req = httptest.NewRequest("GET", "/?order_by=0&limit=5", nil)
	req.Header.Set("AccessToken", ValidToken)
	w = httptest.NewRecorder()
	SearchServer(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	var asIsUsers []User
	_ = json.Unmarshal(w.Body.Bytes(), &asIsUsers)
	if len(asIsUsers) != 5 || asIsUsers[0].ID != 0 {
		t.Errorf("unexpected as-is result: %+v", asIsUsers)
	}
}

func TestSearchServer_RecordContentAccuracy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	client := &SearchClient{
		AccessToken: ValidToken,
		URL:         ts.URL,
	}

	resp, err := client.FindUsers(SearchRequest{
		Limit:      1,
		Offset:     0,
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(resp.Users))
	}

	user0 := resp.Users[0]
	if user0.ID != 0 {
		t.Errorf("expected ID 0, got %d", user0.ID)
	}
	if user0.Name != "Boyd Wolf" {
		t.Errorf("expected Name 'Boyd Wolf', got %q", user0.Name)
	}
	if user0.Age != 22 {
		t.Errorf("expected Age 22, got %d", user0.Age)
	}
	if user0.Gender != "male" {
		t.Errorf("expected Gender 'male', got %q", user0.Gender)
	}
	if !strings.HasPrefix(user0.About, "Nulla cillum enim voluptate") {
		t.Errorf("unexpected About field: %q", user0.About)
	}
	
	expected := User{
		ID:     0,
		Name:   "Boyd Wolf",
		Age:    22,
		Gender: "male",
		About:  user0.About,
	}
	if !reflect.DeepEqual(user0, expected) {
		t.Errorf("user does not match expected:\ngot  %+v\nwant %+v", user0, expected)
	}
}
