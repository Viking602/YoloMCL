package nativeui

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type pendingRequest struct {
	method, page, tab, selection, dialog string
	dialogGen                            uint64
	fields                               map[string]string
}

// All commands run without an application-wide busy state. Controls opt into
// their own request's pending state, so adding a new command cannot dim the
// rest of the window. Read-only commands do not rescan installed instances.
var commandNoSnapshot = map[string]bool{
	"logs": true, "versions": true, "fetchAuthServerName": true,
	"checkPack": true, "packUpdateReport": true, "planPackUpdate": true,
	"inspectPack": true, "previewRemotePack": true, "startMicrosoft": true,
}

func (a *App) pendingOp(methods ...string) bool {
	for _, method := range methods {
		if a.pending[method] > 0 {
			return true
		}
	}
	return false
}

// pendingAt matches a method and optional field/value pairs, for example a
// particular save or account. Unrelated rows remain usable.
func (a *App) pendingAt(method string, fields ...string) bool {
	for _, req := range a.pendingRequests {
		if req.method != method {
			continue
		}
		match := true
		for i := 0; i+1 < len(fields); i += 2 {
			if req.fields[fields[i]] != fields[i+1] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func (a *App) dialogPending() bool {
	if !a.dialogOpen {
		return false
	}
	for _, req := range a.pendingRequests {
		if req.dialog != "" && req.dialog == a.dialog && req.dialogGen == a.dialogGen {
			return true
		}
	}
	return false
}

func (a *App) instancePending(methods ...string) bool {
	r := a.root()
	if r == nil {
		return false
	}
	for _, req := range a.pendingRequests {
		if req.fields["rootId"] != r.ID || (req.fields["version"] != "" && req.fields["version"] != r.SelectedVersion) {
			continue
		}
		for _, method := range methods {
			if req.method == method {
				return true
			}
		}
	}
	return false
}

func (a *App) texturePending(kind string) bool {
	ac := a.account()
	return ac != nil && (a.pendingAt("uploadTexture", "accountId", ac.ID, "kind", kind) || a.pendingAt("resetTexture", "accountId", ac.ID, "kind", kind))
}

func requestFields(payload any) map[string]string {
	fields := map[string]string{}
	for _, key := range []string{"rootId", "version", "name", "path", "accountId", "id", "kind", "source", "projectId", "versionId"} {
		switch p := payload.(type) {
		case map[string]string:
			fields[key] = p[key]
		case map[string]any:
			fields[key], _ = p[key].(string)
		}
	}
	return fields
}

func requestKey(method string, payload any) string {
	identity := payload
	if method == "uploadTexture" {
		// Image data can be several MiB: serialize it only in the worker.
		identity = requestFields(payload)
	}
	b, _ := json.Marshal(identity)
	return fmt.Sprintf("%s:%x", method, sha256.Sum256(b))
}

func (a *App) execute(method string, payload any, done func(json.RawMessage)) {
	key := requestKey(method, payload)
	if _, exists := a.pendingRequests[key]; exists {
		return // a double click must not submit the same operation twice
	}
	if a.pendingRequests == nil {
		a.pendingRequests = map[string]pendingRequest{}
	}
	if a.pending == nil {
		a.pending = map[string]int{}
	}
	req := pendingRequest{method: method, page: a.page, tab: a.instTab, selection: a.versionSwapKey(), dialogGen: a.dialogGen, fields: requestFields(payload)}
	if a.dialogOpen {
		req.dialog = a.dialog
	}
	a.pendingRequests[key] = req
	a.pending[method]++
	epoch := a.selectionEpoch.Load()
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		b, e := json.Marshal(payload)
		var raw string
		if e == nil {
			raw, e = a.service.Dispatch(a.ctx, method, string(b))
		}
		var snap *launcher.Snapshot
		if e == nil && !commandNoSnapshot[method] {
			s := a.service.Snapshot()
			snap = &s
		}
		a.post(func() {
			delete(a.pendingRequests, key)
			a.pending[method]--
			if a.pending[method] == 0 {
				delete(a.pending, method)
			}
			// An old completion may update data, but must not close a newer
			// dialog, change tabs, or overwrite another instance's view.
			current := req.page == a.page && req.tab == a.instTab && req.selection == a.versionSwapKey() && req.dialogGen == a.dialogGen
			if req.dialog != "" {
				current = current && a.dialogOpen && req.dialog == a.dialog
			}
			if snap != nil {
				a.applySnapshot(*snap, epoch)
			}
			if e != nil {
				a.message, a.errorMessage = a.friendlyErr(e), true
				a.noticeFor, a.noticeRetry = a.message, func() { a.execute(method, payload, done) }
				return
			}
			if done != nil && current {
				done(json.RawMessage(raw))
			}
		})
	}()
}
