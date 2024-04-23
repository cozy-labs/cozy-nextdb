package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/labstack/echo/v4"
)

// n is the number of documents in the benchmark
const n = 10_000

// db is the database used for the benchmark (will be created and deleted)
const db = "cozybench/io-cozy-contacts"

// target is the URL of CouchDB or NextDB to target
const target = "http://admin:password@localhost:5984"

// const target = "http://localhost:7654"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error, %s", err)
		os.Exit(1)
	}
}

func run() error {
	generator := newGenerator()

	if err := createDB(); err != nil {
		return fmt.Errorf("cannot create db: %s", err)
	}
	defer func() {
		_ = deleteDB()
	}()

	contacts := prepareContacts(generator)
	if err := insertDocs(contacts); err != nil {
		return fmt.Errorf("cannot insert contacts: %s", err)
	}

	if err := getAllDocs(); err != nil {
		return fmt.Errorf("cannot get all docs: %s", err)
	}

	return nil
}

func createDB() error {
	defer trace("createDB")()
	return makeRequest("PUT", "", nil, nil)
}

func deleteDB() error {
	defer trace("deleteDB")()
	return makeRequest("DELETE", "", nil, nil)
}

func insertDocs(docs []map[string]any) error {
	defer trace("insertDocs")()
	// Docs are inserted one by one, as nextdb doesn't support bulk requests yet
	for _, doc := range docs {
		body, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		if err := makeRequest("POST", "", nil, body); err != nil {
			return err
		}
	}
	return nil
}

func getAllDocs() error {
	defer trace("getAllDocs")()
	q := url.Values{}
	q.Set("include_docs", "true")
	return makeRequest("GET", "_all_docs", q, nil)
}

func trace(msg string) func() {
	started := time.Now()
	return func() {
		fmt.Printf("%s took %s\n", msg, time.Since(started))
	}
}

func makeRequest(method, path string, query url.Values, reqjson []byte) error {
	u, err := url.Parse(target)
	if err != nil {
		return err
	}
	u.Path = url.PathEscape(db) + "/" + path
	u.RawQuery = query.Encode()
	req, err := http.NewRequest(
		method,
		u.String(),
		bytes.NewReader(reqjson),
	)
	if err != nil {
		return err
	}
	req.Header.Add(echo.HeaderAccept, echo.MIMEApplicationJSON)
	if len(reqjson) > 0 {
		req.Header.Add(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, res.Body)
	return res.Body.Close()
}

func newGenerator() faker.Faker {
	seed := time.Now().UnixNano()
	if s := os.Getenv("SEED"); s != "" {
		var err error
		seed, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid seed: %s", err)
			os.Exit(1)
		}
	}
	fmt.Printf("seed = %d\n", seed)

	return faker.NewWithSeed(rand.NewSource(seed))
}

func prepareContacts(generator faker.Faker) []map[string]any {
	var docs []map[string]any
	for i := 0; i < n; i++ {
		docs = append(docs, generateContact(generator.Person()))
	}
	return docs
}

func generateContact(p faker.Person) map[string]any {
	gender := strings.ToLower(p.Gender())
	lastName := p.LastName()
	var firstName string
	if gender == "male" {
		firstName = p.FirstNameMale()
	} else {
		firstName = p.FirstNameFemale()
	}
	fullname := firstName + " " + lastName
	indexed := strings.ToLower(lastName) + strings.ToLower(firstName)

	doc := map[string]any{
		"address":    []any{},
		"birthday":   "",
		"birthplace": "",
		"company":    "",
		"cozy":       []any{},
		"cozyMetadata": map[string]any{
			"createdAt":           "2024-04-22T15:05:58.917Z",
			"createdByApp":        "Contacts",
			"createdByAppVersion": "1.7.0",
			"doctypeVersion":      3,
			"metadataVersion":     1,
			"updatedAt":           "2024-04-22T15:06:21.046Z",
		},
		"displayName": fullname,
		"email":       []any{},
		"fullname":    fullname,
		"gender":      gender,
		"indexes": map[string]any{
			"byFamilyNameGivenNameEmailCozyUrl": indexed,
		},
		"jobTitle": "",
		"name": map[string]any{
			"familyName": lastName,
			"givenName":  firstName,
		},
		"note":  "",
		"phone": []any{},
	}

	return doc
}
