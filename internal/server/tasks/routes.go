package tasks

import (
	"fmt"
	"net/http"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/util"
)

// AddStaticRoutes registers the /admin/tasks endpoints on r. Called from
// internal/commands/routes.go only when ego.server.tasks.enabled is true,
// so the routes don't exist at all when the feature is off.
func AddStaticRoutes(r *router.Router) {
	// Report every loaded task's description, id, active flag, and
	// last-run outcome.
	r.New(defs.AdminTasksPath, GetTasksHandler, http.MethodGet).
		Permissions(defs.RootPermission).
		Class(router.AdminRequestCounter).
		AcceptMedia(defs.TasksMediaType)

	// Start a task immediately, outside its normal schedule. The reserved
	// id "@reload" (ReloadTaskID) is special-cased inside RunTaskHandler to
	// re-scan the tasks directory instead of running a task by that name.
	r.New(defs.AdminTasksIDPath, RunTaskHandler, http.MethodPost).
		Permissions(defs.RootPermission).
		Class(router.AdminRequestCounter)

	// Deactivate a task: patch its file's "active" field to false and
	// clear the in-memory flag, without removing it from the registry.
	r.New(defs.AdminTasksIDPath, DeleteTaskHandler, http.MethodDelete).
		Permissions(defs.RootPermission).
		Class(router.AdminRequestCounter)
}

// GetTasksHandler is the HTTP handler for GET /admin/tasks.
func GetTasksHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	snapshot := Snapshot()

	items := make([]defs.TaskStatus, 0, len(snapshot))

	for _, s := range snapshot {
		items = append(items, defs.TaskStatus{
			Task:       s.Description,
			ID:         s.ID,
			Active:     s.Active,
			Running:    s.Running,
			LastRun:    s.LastRun,
			LastStatus: s.LastStatus,
			Success:    s.Success,
			RunCount:   s.RunCount,
			FailedTest: s.FailedTest,
		})
	}

	response := defs.TasksResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Status:     http.StatusOK,
		Count:      len(items),
		Items:      items,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.TasksMediaType)
	util.WriteJSON(w, session.Response(), http.StatusOK, response)

	return http.StatusOK
}

// RunTaskHandler is the HTTP handler for POST /admin/tasks/{id}. It starts
// the named task immediately and asynchronously, the same way the
// scheduler itself runs a due task, so this request returns right away
// rather than blocking for as long as the task's own timeout allows.
// Its repeat timer restarts from when this run finishes, exactly like a
// normal scheduled run.
//
// The reserved id ReloadTaskID ("@reload") is special-cased to
// ReloadTasksHandler instead: it doesn't name a task at all, it tells the
// task manager to re-scan the tasks directory.
func RunTaskHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	id := data.String(session.URLParts["id"])

	if id == ReloadTaskID {
		return ReloadTasksHandler(session, w, r)
	}

	task, found := Lookup(id)
	if !found {
		return util.ErrorResponse(w, session.ID, errors.Localize(errors.ErrNotFound.Clone().Context(id), session.Language), http.StatusNotFound)
	}

	if !tryClaim(id) {
		return util.ErrorResponse(w, session.ID, errors.Localize(errors.ErrTasksAlreadyRunning.Clone().Context(id), session.Language), http.StatusConflict)
	}

	ui.Log(tasksLogger, "tasks.run.manual", ui.A{"id": id, "user": session.User})

	go runOne(task)

	// 202 is not the framework's default (200 is written implicitly when a
	// handler never calls WriteHeader), so it must be set explicitly.
	w.WriteHeader(http.StatusAccepted)

	return http.StatusAccepted
}

// DeleteTaskHandler is the HTTP handler for DELETE /admin/tasks/{id}. It
// deactivates the task rather than removing it: the on-disk file's
// "active" field is patched to false (comments and formatting preserved),
// and the in-memory flag is cleared so the scheduler stops considering it
// immediately, without waiting for a restart. The task remains loaded and
// still reportable via GET, just inactive -- letting an admin pull a
// problematic task out of rotation until it's diagnosed.
func DeleteTaskHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	id := data.String(session.URLParts["id"])

	task, found := Lookup(id)
	if !found {
		return util.ErrorResponse(w, session.ID, errors.Localize(errors.ErrNotFound.Clone().Context(id), session.Language), http.StatusNotFound)
	}

	if err := deactivateFile(task.Path); err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	setActive(id, false)

	ui.Log(tasksLogger, "tasks.deactivated", ui.A{"id": id, "user": session.User})

	return http.StatusOK
}

// ReloadTasksHandler implements POST /admin/tasks/@reload. It re-scans the
// tasks directory and merges what it finds into the running registry (see
// Reload), so an admin can add, edit, or reactivate a task by editing its
// file, without stopping the server.
func ReloadTasksHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	result, err := Reload()
	if err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	ui.Log(tasksLogger, "tasks.reload", ui.A{
		"user":    session.User,
		"total":   result.Total,
		"new":     result.New,
		"updated": result.Updated,
		"removed": result.Removed,
	})

	response := defs.TasksResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Status:     http.StatusOK,
		Message: fmt.Sprintf("reloaded: %d total, %d new, %d updated, %d removed",
			result.Total, result.New, result.Updated, result.Removed),
		Count: result.Total,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.TasksMediaType)
	util.WriteJSON(w, session.Response(), http.StatusOK, response)

	return http.StatusOK
}
