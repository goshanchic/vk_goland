package main

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
)

const ValidToken = "valid_token"

var DatasetPath = "dataset.xml"

type XMLUser struct {
	ID        int    `xml:"id"`
	Age       int    `xml:"age"`
	FirstName string `xml:"first_name"`
	LastName  string `xml:"last_name"`
	Gender    string `xml:"gender"`
	About     string `xml:"about"`
}

type XMLRoot struct {
	Rows []XMLUser `xml:"row"`
}

func SearchServer(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("AccessToken") != ValidToken {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	file, err := os.Open(DatasetPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer file.Close()

	var root XMLRoot
	if err := xml.NewDecoder(file).Decode(&root); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	users := make([]User, 0, len(root.Rows))
	for _, row := range root.Rows {
		users = append(users, User{
			ID:     row.ID,
			Name:   row.FirstName + " " + row.LastName,
			Age:    row.Age,
			About:  row.About,
			Gender: row.Gender,
		})
	}

	query := r.URL.Query().Get("query")
	if query != "" {
		filtered := make([]User, 0, len(users))
		for _, u := range users {
			if strings.Contains(u.Name, query) || strings.Contains(u.About, query) {
				filtered = append(filtered, u)
			}
		}
		users = filtered
	}

	orderField := r.URL.Query().Get("order_field")
	if orderField == "" {
		orderField = "Name"
	}

	switch strings.ToLower(orderField) {
	case "id":
		orderField = "Id"
	case "age":
		orderField = "Age"
	case "name":
		orderField = "Name"
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		errBytes, _ := json.Marshal(SearchErrorResponse{Error: ErrorBadOrderField})
		_, _ = w.Write(errBytes)
		return
	}

	orderByStr := r.URL.Query().Get("order_by")
	orderBy := OrderByAsIs
	if orderByStr != "" {
		var err error
		orderBy, err = strconv.Atoi(orderByStr)
		if err != nil || (orderBy != OrderByAsc && orderBy != OrderByDesc && orderBy != OrderByAsIs) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			errBytes, _ := json.Marshal(SearchErrorResponse{Error: "bad order_by: must be -1, 0 or 1"})
			_, _ = w.Write(errBytes)
			return
		}
	}

	if orderBy != OrderByAsIs {
		sort.SliceStable(users, func(i, j int) bool {
			switch orderField {
			case "Id":
				if orderBy == OrderByAsc {
					return users[i].ID < users[j].ID
				}
				return users[i].ID > users[j].ID
			case "Age":
				if orderBy == OrderByAsc {
					return users[i].Age < users[j].Age
				}
				return users[i].Age > users[j].Age
			default: // "Name"
				if orderBy == OrderByAsc {
					return users[i].Name < users[j].Name
				}
				return users[i].Name > users[j].Name
			}
		})
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if offset < 0 {
		offset = 0
	}
	if offset > len(users) {
		users = []User{}
	} else {
		users = users[offset:]
	}

	if limit > 0 && len(users) > limit {
		users = users[:limit]
	}

	data, err := json.Marshal(users)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
