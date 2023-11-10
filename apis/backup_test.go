package apis_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"gocloud.dev/blob"
)

func TestBackupsList(t *testing.T) {
	scenarios := []tests.ApiScenario{
		{
			Name:   "unauthorized",
			Method: http.MethodGet,
			Url:    "/api/backups",
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as auth record",
			Method: http.MethodGet,
			Url:    "/api/backups",
			RequestHeaders: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsInR5cGUiOiJhdXRoUmVjb3JkIiwiY29sbGVjdGlvbklkIjoiX3BiX3VzZXJzX2F1dGhfIiwiZXhwIjoyMjA4OTg1MjYxfQ.UwD8JvkbQtXpymT09d7J6fdA0aP9g4FJ1GPh_ggEkzc",
			},
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as admin (empty list)",
			Method: http.MethodGet,
			Url:    "/api/backups",
			RequestHeaders: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhZG1pbiIsImV4cCI6MjIwODk4NTI2MX0.M1m--VOqGyv0d23eeUc0r9xE8ZzHaYVmVFw1VZW6gT8",
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`[]`,
			},
		},
		{
			Name:   "authorized as admin",
			Method: http.MethodGet,
			Url:    "/api/backups",
			RequestHeaders: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhZG1pbiIsImV4cCI6MjIwODk4NTI2MX0.M1m--VOqGyv0d23eeUc0r9xE8ZzHaYVmVFw1VZW6gT8",
			},
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"test1.zip"`,
				`"test2.zip"`,
				`"test3.zip"`,
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestBackupsCreate(t *testing.T) {
	scenarios := []tests.ApiScenario{
		{
			Name:   "unauthorized",
			Method: http.MethodPost,
			Url:    "/api/backups",
			AfterTestFunc: func(t *testing.T, app *tests.TestApp, res *http.Response) {
				ensureNoBackups(t, app)
			},
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as auth record",
			Method: http.MethodPost,
			Url:    "/api/backups",
			RequestHeaders: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsInR5cGUiOiJhdXRoUmVjb3JkIiwiY29sbGVjdGlvbklkIjoiX3BiX3VzZXJzX2F1dGhfIiwiZXhwIjoyMjA4OTg1MjYxfQ.UwD8JvkbQtXpymT09d7J6fdA0aP9g4FJ1GPh_ggEkzc",
			},
			AfterTestFunc: func(t *testing.T, app *tests.TestApp, res *http.Response) {
				ensureNoBackups(t, app)
			},
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as admin (pending backup)",
			Method: http.MethodPost,
			Url:    "/api/backups",
			RequestHeaders: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhZG1pbiIsImV4cCI6MjIwODk4NTI2MX0.M1m--VOqGyv0d23eeUc0r9xE8ZzHaYVmVFw1VZW6gT8",
			},
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				app.Cache().Set(core.CacheKeyActiveBackup, "")
			},
			AfterTestFunc: func(t *testing.T, app *tests.TestApp, res *http.Response) {
				ensureNoBackups(t, app)
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as admin (autogenerated name)",
			Method: http.MethodPost,
			Url:    "/api/backups",
			RequestHeaders: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhZG1pbiIsImV4cCI6MjIwODk4NTI2MX0.M1m--VOqGyv0d23eeUc0r9xE8ZzHaYVmVFw1VZW6gT8",
			},
			AfterTestFunc: func(t *testing.T, app *tests.TestApp, res *http.Response) {
				files, err := getBackupFiles(app)
				if err != nil {
					t.Fatal(err)
				}

				if total := len(files); total != 1 {
					t.Fatalf("Expected 1 backup file, got %d", total)
				}

				expected := "pb_backup_"
				if !strings.HasPrefix(files[0].Key, expected) {
					t.Fatalf("Expected backup file with prefix %q, got %q", expected, files[0].Key)
				}
			},
			ExpectedStatus: 204,
		},
		{
			Name:   "authorized as admin (invalid name)",
			Method: http.MethodPost,
			Url:    "/api/backups",
			Body:   strings.NewReader(`{"name":"!test.zip"}`),
			RequestHeaders: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhZG1pbiIsImV4cCI6MjIwODk4NTI2MX0.M1m--VOqGyv0d23eeUc0r9xE8ZzHaYVmVFw1VZW6gT8",
			},
			AfterTestFunc: func(t *testing.T, app *tests.TestApp, res *http.Response) {
				ensureNoBackups(t, app)
			},
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"data":{`,
				`"name":{"code":"validation_match_invalid"`,
			},
		},
		{
			Name:   "authorized as admin (valid name)",
			Method: http.MethodPost,
			Url:    "/api/backups",
			Body:   strings.NewReader(`{"name":"test.zip"}`),
			RequestHeaders: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhZG1pbiIsImV4cCI6MjIwODk4NTI2MX0.M1m--VOqGyv0d23eeUc0r9xE8ZzHaYVmVFw1VZW6gT8",
			},
			AfterTestFunc: func(t *testing.T, app *tests.TestApp, res *http.Response) {
				files, err := getBackupFiles(app)
				if err != nil {
					t.Fatal(err)
				}

				if total := len(files); total != 1 {
					t.Fatalf("Expected 1 backup file, got %d", total)
				}

				expected := "test.zip"
				if files[0].Key != expected {
					t.Fatalf("Expected backup file %q, got %q", expected, files[0].Key)
				}
			},
			ExpectedStatus: 204,
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestBackupsUpload(t *testing.T) {
	// create dummy form data bodies
	type body struct {
		buffer      io.Reader
		contentType string
	}
	bodies := make([]body, 10)
	for i := 0; i < 10; i++ {
		func() {
			zb := new(bytes.Buffer)
			zw := zip.NewWriter(zb)
			if err := zw.Close(); err != nil {
				t.Fatal(err)
			}

			b := new(bytes.Buffer)
			mw := multipart.NewWriter(b)

			mfw, err := mw.CreateFormFile("file", "test")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.Copy(mfw, zb); err != nil {
				t.Fatal(err)
			}

			mw.Close()

			bodies[i] = body{
				buffer:      b,
				contentType: mw.FormDataContentType(),
			}
		}()
	}
	// ---

	scenarios := []tests.ApiScenario{
		{
			Name:   "unauthorized",
			Method: http.MethodPost,
			Url:    "/api/backups/upload",
			Body:   bodies[0].buffer,
			RequestHeaders: map[string]string{
				"Content-Type": bodies[0].contentType,
			},
			AfterTestFunc: func(t *testing.T, app *tests.TestApp, res *http.Response) {
				ensureNoBackups(t, app)
			},
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as auth record",
			Method: http.MethodPost,
			Url:    "/api/backups/upload",
			Body:   bodies[1].buffer,
			RequestHeaders: map[string]string{
				"Content-Type":  bodies[1].contentType,
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsInR5cGUiOiJhdXRoUmVjb3JkIiwiY29sbGVjdGlvbklkIjoiX3BiX3VzZXJzX2F1dGhfIiwiZXhwIjoyMjA4OTg1MjYxfQ.UwD8JvkbQtXpymT09d7J6fdA0aP9g4FJ1GPh_ggEkzc",
			},
			AfterTestFunc: func(t *testing.T, app *tests.TestApp, res *http.Response) {
				ensureNoBackups(t, app)
			},
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as admin (missing file)",
			Method: http.MethodPost,
			Url:    "/api/backups/upload",
			RequestHeaders: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhZG1pbiIsImV4cCI6MjIwODk4NTI2MX0.M1m--VOqGyv0d23eeUc0r9xE8ZzHaYVmVFw1VZW6gT8",
			},
			AfterTestFunc: func(t *testing.T, app *tests.TestApp, res *http.Response) {
				ensureNoBackups(t, app)
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{`},
		},
		{
			Name:   "authorized as admin (existing backup name)",
			Method: http.MethodPost,
			Url:    "/api/backups/upload",
			Body:   bodies[3].buffer,
			RequestHeaders: map[string]string{
				"Content-Type":  bodies[3].contentType,
				"Authorization": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhZG1pbiIsImV4cCI6MjIwODk4NTI2MX0.M1m--VOqGyv0d23eeUc0r9xE8ZzHaYVmVFw1VZW6gT8",
			},
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				fsys, err := app.NewBackupsFilesystem()
				if err != nil {
					t.Fatal(err)
				}
				defer fsys.Close()
				// create a dummy backup file to simulate existing backups
				if err := fsys.Upload([]byte("123"), "test"); err != nil {
					t.Fatal(err)
				}
			},
			AfterTestFunc: func(t *testing.T, app *tests.TestApp, res *http.Response) {
				files, _ := getBackupFiles(app)
				if total := len(files); total != 1 {
					t.Fatalf("Expected %d backup file, got %d", 1, total)
				}
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{"file":{`},
		},
		{
			Name:   "authorized as admin (valid file)",
			Method: http.MethodPost,
			Url:    "/api/backups/upload",
			Body:   bodies[4].buffer,
			RequestHeaders: map[string]string{
				"Content-Type":  bodies[4].contentType,
				"Authorization": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhZG1pbiIsImV4cCI6MjIwODk4NTI2MX0.M1m--VOqGyv0d23eeUc0r9xE8ZzHaYVmVFw1VZW6gT8",
			},
			AfterTestFunc: func(t *testing.T, app *tests.TestApp, res *http.Response) {
				files, _ := getBackupFiles(app)
				if total := len(files); total != 1 {
					t.Fatalf("Expected %d backup file, got %d", 1, total)
				}
			},
			ExpectedStatus: 204,
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestBackupsDownload(t *testing.T) {
	scenarios := []tests.ApiScenario{
		{
			Name:   "unauthorized",
			Method: http.MethodGet,
			Url:    "/api/backups/test1.zip",
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "with record auth header",
			Method: http.MethodGet,
			Url:    "/api/backups/test1.zip",
			RequestHeaders: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsInR5cGUiOiJhdXRoUmVjb3JkIiwiY29sbGVjdGlvbklkIjoiX3BiX3VzZXJzX2F1dGhfIiwiZXhwIjoyMjA4OTg1MjYxfQ.UwD8JvkbQtXpymT09d7J6fdA0aP9g4FJ1GPh_ggEkzc",
			},
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "with admin auth header",
			Method: http.MethodGet,
			Url:    "/api/backups/test1.zip",
			RequestHeaders: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhZG1pbiIsImV4cCI6MjIwODk4NTI2MX0.M1m--VOqGyv0d23eeUc0r9xE8ZzHaYVmVFw1VZW6gT8",
			},
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "with empty or invalid token",
			Method: http.MethodGet,
			Url:    "/api/backups/test1.zip?token=",
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "with valid record auth token",
			Method: http.MethodGet,
			Url:    "/api/backups/test1.zip?token=eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsInR5cGUiOiJhdXRoUmVjb3JkIiwiY29sbGVjdGlvbklkIjoiX3BiX3VzZXJzX2F1dGhfIiwiZXhwIjoyMjA4OTg1MjYxfQ.UwD8JvkbQtXpymT09d7J6fdA0aP9g4FJ1GPh_ggEkzc",
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "with valid record file token",
			Method: http.MethodGet,
			Url:    "/api/backups/test1.zip?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsImV4cCI6MTg5MzQ1MjQ2MSwiY29sbGVjdGlvbklkIjoiX3BiX3VzZXJzX2F1dGhfIiwidHlwZSI6ImF1dGhSZWNvcmQifQ.0d_0EO6kfn9ijZIQWAqgRi8Bo1z7MKcg1LQpXhQsEPk",
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "with valid admin auth token",
			Method: http.MethodGet,
			Url:    "/api/backups/test1.zip?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhZG1pbiIsImV4cCI6MjIwODk4NTI2MX0.M1m--VOqGyv0d23eeUc0r9xE8ZzHaYVmVFw1VZW6gT8",
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "with expired admin file token",
			Method: http.MethodGet,
			Url:    "/api/backups/test1.zip?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsImV4cCI6MTY0MDk5MTY2MSwidHlwZSI6ImFkbWluIn0.g7Q_3UX6H--JWJ7yt1Hoe-1ugTX1KpbKzdt0zjGSe-E",
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "with valid admin file token but missing backup name",
			Method: http.MethodGet,
			Url:    "/api/backups/missing?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsImV4cCI6MTg5MzQ1MjQ2MSwidHlwZSI6ImFkbWluIn0.LyAMpSfaHVsuUqIlqqEbhDQSdFzoPz_EIDcb2VJMBsU",
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "with valid admin file token",
			Method: http.MethodGet,
			Url:    "/api/backups/test1.zip?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsImV4cCI6MTg5MzQ1MjQ2MSwidHlwZSI6ImFkbWluIn0.LyAMpSfaHVsuUqIlqqEbhDQSdFzoPz_EIDcb2VJMBsU",
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`storage/`,
				`data.db`,
				`logs.db`,
			},
		},
		{
			Name:   "with valid admin file token and backup name with escaped char",
			Method: http.MethodGet,
			Url:    "/api/backups/%40test4.zip?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsImV4cCI6MTg5MzQ1MjQ2MSwidHlwZSI6ImFkbWluIn0.LyAMpSfaHVsuUqIlqqEbhDQSdFzoPz_EIDcb2VJMBsU",
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`storage/`,
				`data.db`,
				`logs.db`,
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestBackupsDelete(t *testing.T) {
	noTestBackupFilesChanges := func(t *testing.T, app *tests.TestApp) {
		files, err := getBackupFiles(app)
		if err != nil {
			t.Fatal(err)
		}

		expected := 4
		if total := len(files); total != expected {
			t.Fatalf("Expected %d backup(s), got %d", expected, total)
		}
	}

	scenarios := []tests.ApiScenario{
		{
			Name:   "unauthorized",
			Method: http.MethodDelete,
			Url:    "/api/backups/test1.zip",
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			AfterTestFunc: func(t *testing.T, app *tests.TestApp, res *http.Response) {
				noTestBackupFilesChanges(t, app)
			},
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as auth record",
			Method: http.MethodDelete,
			Url:    "/api/backups/test1.zip",
			RequestHeaders: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsInR5cGUiOiJhdXRoUmVjb3JkIiwiY29sbGVjdGlvbklkIjoiX3BiX3VzZXJzX2F1dGhfIiwiZXhwIjoyMjA4OTg1MjYxfQ.UwD8JvkbQtXpymT09d7J6fdA0aP9g4FJ1GPh_ggEkzc",
			},
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			AfterTestFunc: func(t *testing.T, app *tests.TestApp, res *http.Response) {
				noTestBackupFilesChanges(t, app)
			},
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as admin (missing file)",
			Method: http.MethodDelete,
			Url:    "/api/backups/missing.zip",
			RequestHeaders: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhZG1pbiIsImV4cCI6MjIwODk4NTI2MX0.M1m--VOqGyv0d23eeUc0r9xE8ZzHaYVmVFw1VZW6gT8",
			},
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			AfterTestFunc: func(t *testing.T, app *tests.TestApp, res *http.Response) {
				noTestBackupFilesChanges(t, app)
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as admin (existing file with matching active backup)",
			Method: http.MethodDelete,
			Url:    "/api/backups/test1.zip",
			RequestHeaders: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhZG1pbiIsImV4cCI6MjIwODk4NTI2MX0.M1m--VOqGyv0d23eeUc0r9xE8ZzHaYVmVFw1VZW6gT8",
			},
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}

				// mock active backup with the same name to delete
				app.Cache().Set(core.CacheKeyActiveBackup, "test1.zip")
			},
			AfterTestFunc: func(t *testing.T, app *tests.TestApp, res *http.Response) {
				noTestBackupFilesChanges(t, app)
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as admin (existing file and no matching active backup)",
			Method: http.MethodDelete,
			Url:    "/api/backups/test1.zip",
			RequestHeaders: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhZG1pbiIsImV4cCI6MjIwODk4NTI2MX0.M1m--VOqGyv0d23eeUc0r9xE8ZzHaYVmVFw1VZW6gT8",
			},
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}

				// mock active backup with different name
				app.Cache().Set(core.CacheKeyActiveBackup, "new.zip")
			},
			AfterTestFunc: func(t *testing.T, app *tests.TestApp, res *http.Response) {
				files, err := getBackupFiles(app)
				if err != nil {
					t.Fatal(err)
				}

				if total := len(files); total != 3 {
					t.Fatalf("Expected %d backup files, got %d", 3, total)
				}

				deletedFile := "test1.zip"

				for _, f := range files {
					if f.Key == deletedFile {
						t.Fatalf("Expected backup %q to be deleted", deletedFile)
					}
				}
			},
			ExpectedStatus: 204,
		},
		{
			Name:   "authorized as admin (backup with escaped character)",
			Method: http.MethodDelete,
			Url:    "/api/backups/%40test4.zip",
			RequestHeaders: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhZG1pbiIsImV4cCI6MjIwODk4NTI2MX0.M1m--VOqGyv0d23eeUc0r9xE8ZzHaYVmVFw1VZW6gT8",
			},
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			AfterTestFunc: func(t *testing.T, app *tests.TestApp, res *http.Response) {
				files, err := getBackupFiles(app)
				if err != nil {
					t.Fatal(err)
				}

				if total := len(files); total != 3 {
					t.Fatalf("Expected %d backup files, got %d", 3, total)
				}

				deletedFile := "@test4.zip"

				for _, f := range files {
					if f.Key == deletedFile {
						t.Fatalf("Expected backup %q to be deleted", deletedFile)
					}
				}
			},
			ExpectedStatus: 204,
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestBackupsRestore(t *testing.T) {
	scenarios := []tests.ApiScenario{
		{
			Name:   "unauthorized",
			Method: http.MethodPost,
			Url:    "/api/backups/test1.zip/restore",
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as auth record",
			Method: http.MethodPost,
			Url:    "/api/backups/test1.zip/restore",
			RequestHeaders: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsInR5cGUiOiJhdXRoUmVjb3JkIiwiY29sbGVjdGlvbklkIjoiX3BiX3VzZXJzX2F1dGhfIiwiZXhwIjoyMjA4OTg1MjYxfQ.UwD8JvkbQtXpymT09d7J6fdA0aP9g4FJ1GPh_ggEkzc",
			},
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as admin (missing file)",
			Method: http.MethodPost,
			Url:    "/api/backups/missing.zip/restore",
			RequestHeaders: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhZG1pbiIsImV4cCI6MjIwODk4NTI2MX0.M1m--VOqGyv0d23eeUc0r9xE8ZzHaYVmVFw1VZW6gT8",
			},
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as admin (active backup process)",
			Method: http.MethodPost,
			Url:    "/api/backups/test1.zip/restore",
			RequestHeaders: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhZG1pbiIsImV4cCI6MjIwODk4NTI2MX0.M1m--VOqGyv0d23eeUc0r9xE8ZzHaYVmVFw1VZW6gT8",
			},
			BeforeTestFunc: func(t *testing.T, app *tests.TestApp, e *echo.Echo) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}

				app.Cache().Set(core.CacheKeyActiveBackup, "")
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{}`},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

// -------------------------------------------------------------------

func createTestBackups(app core.App) error {
	ctx := context.Background()

	if err := app.CreateBackup(ctx, "test1.zip"); err != nil {
		return err
	}

	if err := app.CreateBackup(ctx, "test2.zip"); err != nil {
		return err
	}

	if err := app.CreateBackup(ctx, "test3.zip"); err != nil {
		return err
	}

	if err := app.CreateBackup(ctx, "@test4.zip"); err != nil {
		return err
	}

	return nil
}

func getBackupFiles(app core.App) ([]*blob.ListObject, error) {
	fsys, err := app.NewBackupsFilesystem()
	if err != nil {
		return nil, err
	}
	defer fsys.Close()

	return fsys.List("")
}

func ensureNoBackups(t *testing.T, app *tests.TestApp) {
	files, err := getBackupFiles(app)
	if err != nil {
		t.Fatal(err)
	}

	if total := len(files); total != 0 {
		t.Fatalf("Expected 0 backup files, got %d", total)
	}
}
