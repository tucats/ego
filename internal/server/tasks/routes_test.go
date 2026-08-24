package tasks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/language/tokens"
	"github.com/tucats/ego/internal/router"
)

func adminToken(t *testing.T) string {
	t.Helper()

	token, err := tokens.New(defs.DefaultAdminUsername, "", tokenTTL, defs.InstanceID, 0)
	if err != nil {
		t.Fatalf("failed to mint test token: %v", err)
	}

	return token
}

func doAdminRequest(t *testing.T, method, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", defs.AuthScheme+adminToken(t))
	req.Header.Set("Accept", defs.JSONMediaType)

	rec := httptest.NewRecorder()
	router.ServerRouter.ServeHTTP(rec, req)

	return rec
}

func TestGetTasksHandlerReportsSnapshot(t *testing.T) {
	resetRegistry(t)

	r := useTestRouter(t)
	AddStaticRoutes(r)

	registry["t1"] = &Task{ID: "t1", Description: "first task", Active: true}
	states["t1"] = &State{LastStatus: 200, Success: true}

	registry["t2"] = &Task{ID: "t2", Description: "second task", Active: false}
	states["t2"] = &State{}

	rec := doAdminRequest(t, http.MethodGet, defs.AdminTasksPath)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var response defs.TasksResponse

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v, body=%s", err, rec.Body.String())
	}

	if response.Count != 2 {
		t.Fatalf("count = %d, want 2", response.Count)
	}

	byID := map[string]defs.TaskStatus{}
	for _, item := range response.Items {
		byID[item.ID] = item
	}

	first, found := byID["t1"]
	if !found {
		t.Fatal("expected t1 in the response")
	}

	if first.Task != "first task" || !first.Active || first.LastStatus != 200 || !first.Success {
		t.Errorf("t1 = %+v, did not match expected fields", first)
	}

	second, found := byID["t2"]
	if !found {
		t.Fatal("expected t2 in the response")
	}

	if second.Active {
		t.Error("t2 should be reported as inactive")
	}
}

func TestRunTaskHandlerStartsAsyncAndReturnsAccepted(t *testing.T) {
	resetRegistry(t)

	r := useTestRouter(t)
	AddStaticRoutes(r)

	registry["t1"] = &Task{ID: "t1", Active: true}
	states["t1"] = &State{}

	release := make(chan struct{})

	var started int32

	useFakeDispatcher(t, func(*Task) (int, bool) {
		started++
		<-release

		return 200, true
	})

	rec := doAdminRequest(t, http.MethodPost, defs.AdminTasksPath+"t1")

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	deadline := time.Now().Add(2 * time.Second)
	for runningCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if runningCount() != 1 {
		t.Errorf("runningCount() = %d, want 1 (the manually started task)", runningCount())
	}

	close(release)

	deadline = time.Now().Add(2 * time.Second)
	for runningCount() > 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if runningCount() != 0 {
		t.Fatalf("runningCount() = %d after release, want 0", runningCount())
	}
}

func TestRunTaskHandlerUnknownIDReturnsNotFound(t *testing.T) {
	resetRegistry(t)

	r := useTestRouter(t)
	AddStaticRoutes(r)

	rec := doAdminRequest(t, http.MethodPost, defs.AdminTasksPath+"no-such-id")

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRunTaskHandlerAlreadyRunningReturnsConflict(t *testing.T) {
	resetRegistry(t)

	r := useTestRouter(t)
	AddStaticRoutes(r)

	registry["t1"] = &Task{ID: "t1", Active: true}
	states["t1"] = &State{Running: true}

	rec := doAdminRequest(t, http.MethodPost, defs.AdminTasksPath+"t1")

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestDeleteTaskHandlerDeactivatesFileAndMemory(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	path := writeTaskFile(t, dir, "example.json", "# a comment\n"+validTaskJSON)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := useTestRouter(t)
	AddStaticRoutes(r)

	const id = "11111111-1111-1111-1111-111111111111"

	rec := doAdminRequest(t, http.MethodDelete, defs.AdminTasksPath+id)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	task, found := Lookup(id)
	if !found {
		t.Fatal("expected the task to remain registered after deactivation")
	}

	if task.Active {
		t.Error("expected task.Active to be false after DELETE")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	if !strings.Contains(string(content),`"active": "false"`) {
		t.Errorf("file content was not patched to active:false:\n%s", content)
	}

	if !strings.Contains(string(content),"# a comment") {
		t.Errorf("file comment was lost during deactivation:\n%s", content)
	}
}

func TestDeleteTaskHandlerUnknownIDReturnsNotFound(t *testing.T) {
	resetRegistry(t)

	r := useTestRouter(t)
	AddStaticRoutes(r)

	rec := doAdminRequest(t, http.MethodDelete, defs.AdminTasksPath+"no-such-id")

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
