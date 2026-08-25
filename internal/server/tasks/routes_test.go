package tasks

import (
	"bytes"
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

// doAdminBodyRequest is doAdminRequest's counterpart for a request that
// carries a JSON body, such as PATCH /admin/tasks/{id}.
func doAdminBodyRequest(t *testing.T, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Authorization", defs.AuthScheme+adminToken(t))
	req.Header.Set("Accept", defs.JSONMediaType)
	req.Header.Set("Content-Type", defs.JSONMediaType)

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

	useFakeDispatcher(t, func(*Task) (int, bool, string) {
		started++
		
		<-release

		return 200, true, ""
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

	if !strings.Contains(string(content), `"active": "false"`) {
		t.Errorf("file content was not patched to active:false:\n%s", content)
	}

	if !strings.Contains(string(content), "# a comment") {
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

func TestRunTaskHandlerReloadTriggersRescan(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := useTestRouter(t)
	AddStaticRoutes(r)

	// No task files exist yet, so a POST to the reserved id should
	// succeed (not 404, even though "@reload" doesn't name a real task)
	// and report zero tasks found.
	rec := doAdminRequest(t, http.MethodPost, defs.AdminTasksPath+ReloadTaskID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var response defs.TasksResponse

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v, body=%s", err, rec.Body.String())
	}

	if response.Count != 0 {
		t.Errorf("count = %d, want 0 before any task file exists", response.Count)
	}

	// Now add a task file on disk (as an admin editing lib/tasks/ directly
	// would) and reload again -- it should be picked up without a restart.
	writeTaskFile(t, dir, "example.json", validTaskJSON)

	rec = doAdminRequest(t, http.MethodPost, defs.AdminTasksPath+ReloadTaskID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v, body=%s", err, rec.Body.String())
	}

	if response.Count != 1 {
		t.Errorf("count = %d, want 1 after adding a task file and reloading", response.Count)
	}

	if _, found := Lookup("11111111-1111-1111-1111-111111111111"); !found {
		t.Error("expected the new task to be registered after POST @reload")
	}
}

func TestPatchTaskHandlerAppliesPatchAndReturnsUpdatedDetail(t *testing.T) {
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

	body := []byte(`{"active": false, "interval": "5m", "count": 3}`)

	rec := doAdminBodyRequest(t, http.MethodPatch, defs.AdminTasksPath+id, body)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var response defs.TaskDetailResponse

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v, body=%s", err, rec.Body.String())
	}

	if response.Active {
		t.Error("response.Active = true, want false")
	}

	if response.Interval != "5m" {
		t.Errorf("response.Interval = %q, want %q", response.Interval, "5m")
	}

	if response.Count != 3 {
		t.Errorf("response.Count = %d, want 3", response.Count)
	}

	// Fields not touched by the patch must still be reported.
	if response.Endpoint != "/services/jiggle" {
		t.Errorf("response.Endpoint = %q, unexpectedly changed", response.Endpoint)
	}

	task, found := Lookup(id)
	if !found {
		t.Fatal("task disappeared from the registry after PATCH")
	}

	if task.Active || task.Interval != "5m" || task.Count != 3 {
		t.Errorf("registered task after PATCH = %+v, did not take the patch", task)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	text := string(content)

	if !strings.Contains(text, `"active": "false"`) || !strings.Contains(text, `"interval": "5m"`) || !strings.Contains(text, `"count": 3`) {
		t.Errorf("file was not patched as expected:\n%s", text)
	}

	if !strings.Contains(text, "# a comment") {
		t.Errorf("file comment was lost during PATCH:\n%s", text)
	}
}

func TestPatchTaskHandlerUnknownIDReturnsNotFound(t *testing.T) {
	resetRegistry(t)

	r := useTestRouter(t)
	AddStaticRoutes(r)

	rec := doAdminBodyRequest(t, http.MethodPatch, defs.AdminTasksPath+"no-such-id", []byte(`{"active": false}`))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestPatchTaskHandlerNoFieldsReturnsBadRequest(t *testing.T) {
	resetRegistry(t)

	r := useTestRouter(t)
	AddStaticRoutes(r)

	registry["t1"] = &Task{ID: "t1", Active: true, Interval: "10s"}
	states["t1"] = &State{}

	rec := doAdminBodyRequest(t, http.MethodPatch, defs.AdminTasksPath+"t1", []byte(`{}`))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestPatchTaskHandlerDisallowedFieldReturnsBadRequest(t *testing.T) {
	resetRegistry(t)

	r := useTestRouter(t)
	AddStaticRoutes(r)

	registry["t1"] = &Task{ID: "t1", Active: true, Method: "POST", Endpoint: "/services/jiggle"}
	states["t1"] = &State{}

	// "endpoint" is a real Task field, but not one of the four this
	// endpoint is allowed to change -- it must be rejected outright, not
	// silently ignored (which could otherwise look like a successful
	// no-op patch to a caller who mistakenly thought they'd changed it).
	rec := doAdminBodyRequest(t, http.MethodPatch, defs.AdminTasksPath+"t1", []byte(`{"endpoint": "/services/other"}`))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	task, _ := Lookup("t1")
	if task.Endpoint != "/services/jiggle" {
		t.Errorf("task.Endpoint = %q, should not have been changed by a rejected patch", task.Endpoint)
	}
}

func TestPatchTaskHandlerMalformedBodyReturnsBadRequest(t *testing.T) {
	resetRegistry(t)

	r := useTestRouter(t)
	AddStaticRoutes(r)

	registry["t1"] = &Task{ID: "t1", Active: true}
	states["t1"] = &State{}

	rec := doAdminBodyRequest(t, http.MethodPatch, defs.AdminTasksPath+"t1", []byte(`not json`))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestPatchTaskHandlerInvalidIntervalReturnsBadRequestAndLeavesFileUnchanged(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	path := writeTaskFile(t, dir, "example.json", validTaskJSON)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := useTestRouter(t)
	AddStaticRoutes(r)

	const id = "11111111-1111-1111-1111-111111111111"

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}

	rec := doAdminBodyRequest(t, http.MethodPatch, defs.AdminTasksPath+id, []byte(`{"interval": "not-a-duration"}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	if string(after) != string(before) {
		t.Errorf("file was modified despite a validation failure:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestPatchTaskHandlerReactivatesADeactivatedTask(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	writeTaskFile(t, dir, "example.json", validTaskJSON)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := useTestRouter(t)
	AddStaticRoutes(r)

	const id = "11111111-1111-1111-1111-111111111111"

	// First deactivate via DELETE (the existing, one-directional path)...
	if rec := doAdminRequest(t, http.MethodDelete, defs.AdminTasksPath+id); rec.Code != http.StatusOK {
		t.Fatalf("DELETE status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if task, _ := Lookup(id); task.Active {
		t.Fatal("test setup: task should be inactive after DELETE")
	}

	// ...then confirm PATCH can reverse it, unlike DELETE which only ever
	// deactivates.
	rec := doAdminBodyRequest(t, http.MethodPatch, defs.AdminTasksPath+id, []byte(`{"active": true}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	task, found := Lookup(id)
	if !found {
		t.Fatal("task disappeared from the registry")
	}

	if !task.Active {
		t.Error("expected task.Active to be true after PATCH active:true")
	}
}
