package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminClientRunningParsesPids(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/admin/vms/running", r.URL.Path)
		_, _ = w.Write([]byte(`{"pids":["pid-1","pid-2"]}`))
	}))
	defer server.Close()

	pids, err := newAdminClient(server.URL).Running()

	require.NoError(t, err)
	require.Equal(t, []string{"pid-1", "pid-2"}, pids)
}

func TestAdminClientStopResumePayloads(t *testing.T) {
	seen := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		var req adminVMRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, "pid-1", req.Pid)
		seen = append(seen, r.URL.Path)
		switch r.URL.Path {
		case "/admin/vms/stop":
			_, _ = w.Write([]byte(`{"id":"pid-1","message":"stopped"}`))
		case "/admin/vms/resume":
			_, _ = w.Write([]byte(`{"id":"pid-1","message":"resumed"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newAdminClient(server.URL)
	require.NoError(t, client.Stop("pid-1"))
	require.NoError(t, client.Resume("pid-1"))
	require.Equal(t, []string{"/admin/vms/stop", "/admin/vms/resume"}, seen)
}

func TestAdminClientReturnsErrorEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"err_process_stopped"}`))
	}))
	defer server.Close()

	err := newAdminClient(server.URL).Stop("pid-1")

	require.Error(t, err)
	require.Contains(t, err.Error(), "err_process_stopped")
}

func TestRunStopResumeCmdPrintsLifecycleEvidence(t *testing.T) {
	admin := &fakeStopResumeAdmin{pid: "pid-1"}
	sends := 0
	sendEcho := func(pid, plain string) (map[string]string, error) {
		admin.events = append(admin.events, "send")
		sends++
		require.Equal(t, "pid-1", pid)
		require.Equal(t, stopResumePlain, plain)
		if sends == 1 {
			return nil, errProcessStoppedForTest{}
		}
		return map[string]string{"Secret": "", "Plain": plain}, nil
	}
	var buf bytes.Buffer

	err := runStopResumeCmd(&buf, []string{"pid-1"}, admin, sendEcho)

	require.NoError(t, err)
	require.Equal(t, []string{"running", "stop", "running", "send", "resume", "running", "send"}, admin.events)
	require.Equal(t, 2, sends)
	output := buf.String()
	require.Contains(t, output, "STOP_RESUME running_before=true")
	require.Contains(t, output, "STOP_RESUME stopped=true")
	require.Contains(t, output, "STOP_RESUME running_after_stop=false")
	require.Contains(t, output, "STOP_RESUME stopped_send_rejected=true")
	require.Contains(t, output, "STOP_RESUME resumed=true")
	require.Contains(t, output, "STOP_RESUME running_after_resume=true")
	require.Contains(t, output, "STOP_RESUME resume_decrypted=true")
}

func TestRunStopResumeCmdRejectsUnexpectedStoppedSendError(t *testing.T) {
	admin := &fakeStopResumeAdmin{pid: "pid-1"}
	sendEcho := func(pid, plain string) (map[string]string, error) {
		return nil, errUnexpectedSendFailureForTest{}
	}

	err := runStopResumeCmd(&bytes.Buffer{}, []string{"pid-1"}, admin, sendEcho)

	require.Error(t, err)
	require.Contains(t, err.Error(), "expected stopped process error")
}

func TestIsStoppedSendError(t *testing.T) {
	require.True(t, isStoppedSendError(errProcessStoppedForTest{}))
	require.False(t, isStoppedSendError(errHTTPBadRequestForTest{}))
	require.False(t, isStoppedSendError(errUnexpectedSendFailureForTest{}))
	require.False(t, isStoppedSendError(nil))
}

func TestStopResumeCmdRequiresPid(t *testing.T) {
	err := runStopResumeCmd(&bytes.Buffer{}, nil, &fakeStopResumeAdmin{pid: "pid-1"}, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "usage")
}

type fakeStopResumeAdmin struct {
	pid     string
	stopped bool
	events  []string
}

func (f *fakeStopResumeAdmin) Running() ([]string, error) {
	f.events = append(f.events, "running")
	if f.stopped {
		return []string{}, nil
	}
	return []string{f.pid}, nil
}

func (f *fakeStopResumeAdmin) Stop(pid string) error {
	f.events = append(f.events, "stop")
	f.stopped = true
	return nil
}

func (f *fakeStopResumeAdmin) Resume(pid string) error {
	f.events = append(f.events, "resume")
	f.stopped = false
	return nil
}

type errProcessStoppedForTest struct{}

func (errProcessStoppedForTest) Error() string { return "err_process_stopped" }

type errHTTPBadRequestForTest struct{}

func (errHTTPBadRequestForTest) Error() string { return "invalid server response: 400" }

type errUnexpectedSendFailureForTest struct{}

func (errUnexpectedSendFailureForTest) Error() string { return "connection refused" }
