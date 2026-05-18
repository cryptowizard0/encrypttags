package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	hymxSchema "github.com/hymatrix/hymx/schema"
	serverSchema "github.com/hymatrix/hymx/server/schema"
	"github.com/hymatrix/hymx/utils"
	goarSchema "github.com/permadao/goar/schema"
)

const (
	defaultAdminURL = "http://127.0.0.1:8081"
	stopResumePlain = "stop-resume-plain-e2e"
)

var adminURL = envOrDefault("ENCRYPTTAGS_ADMIN_URL", defaultAdminURL)

type stopResumeAdmin interface {
	Running() ([]string, error)
	Stop(pid string) error
	Resume(pid string) error
}

type echoSender func(pid, plain string) (map[string]string, error)

type adminClient struct {
	baseURL    string
	httpClient *http.Client
}

type adminVMRequest struct {
	Pid string `json:"pid"`
}

type adminVMResponse struct {
	Id      string `json:"id"`
	Message string `json:"message"`
}

type adminRunningResponse struct {
	Pids []string `json:"pids"`
}

type adminErrorResponse struct {
	Error string `json:"error"`
}

func newAdminClient(baseURL string) *adminClient {
	return &adminClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: http.DefaultClient,
	}
}

func (c *adminClient) Running() ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/admin/vms/running", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, adminHTTPError(resp)
	}
	var running adminRunningResponse
	if err := json.NewDecoder(resp.Body).Decode(&running); err != nil {
		return nil, fmt.Errorf("decode running vms: %w", err)
	}
	return running.Pids, nil
}

func (c *adminClient) Stop(pid string) error {
	return c.postVM("/admin/vms/stop", pid, "stopped")
}

func (c *adminClient) Resume(pid string) error {
	return c.postVM("/admin/vms/resume", pid, "resumed")
}

func (c *adminClient) postVM(path, pid, expectedMessage string) error {
	payload, err := json.Marshal(adminVMRequest{Pid: pid})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return adminHTTPError(resp)
	}
	var result adminVMResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode admin response: %w", err)
	}
	if result.Id != pid || result.Message != expectedMessage {
		return fmt.Errorf("unexpected admin response: id=%s message=%s", result.Id, result.Message)
	}
	return nil
}

func adminHTTPError(resp *http.Response) error {
	var adminErr adminErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&adminErr); err == nil && adminErr.Error != "" {
		return errors.New(adminErr.Error)
	}
	return fmt.Errorf("invalid admin response: %d", resp.StatusCode)
}

func stopResumeCmd(w io.Writer, args []string) error {
	return runStopResumeCmd(w, args, newAdminClient(adminURL), sendPlainEchoForStopResume)
}

func runStopResumeCmd(w io.Writer, args []string, admin stopResumeAdmin, sendEcho echoSender) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: stop-resume <pid>")
	}
	pid := args[0]

	running, err := admin.Running()
	if err != nil {
		return fmt.Errorf("list running vms before stop: %w", err)
	}
	runningBefore := containsPid(running, pid)
	fmt.Fprintf(w, "STOP_RESUME running_before=%v\n", runningBefore)
	if !runningBefore {
		return fmt.Errorf("process %s is not running before stop", pid)
	}

	if err := admin.Stop(pid); err != nil {
		return fmt.Errorf("stop vm: %w", err)
	}
	fmt.Fprintln(w, "STOP_RESUME stopped=true")

	running, err = admin.Running()
	if err != nil {
		return fmt.Errorf("list running vms after stop: %w", err)
	}
	runningAfterStop := containsPid(running, pid)
	fmt.Fprintf(w, "STOP_RESUME running_after_stop=%v\n", runningAfterStop)
	if runningAfterStop {
		return fmt.Errorf("process %s is still running after stop", pid)
	}

	if _, err := sendEcho(pid, stopResumePlain); err == nil {
		return fmt.Errorf("stopped process accepted message")
	} else if !isStoppedSendError(err) {
		return fmt.Errorf("expected stopped process error, got: %w", err)
	}
	fmt.Fprintln(w, "STOP_RESUME stopped_send_rejected=true")

	if err := admin.Resume(pid); err != nil {
		return fmt.Errorf("resume vm: %w", err)
	}
	fmt.Fprintln(w, "STOP_RESUME resumed=true")

	running, err = admin.Running()
	if err != nil {
		return fmt.Errorf("list running vms after resume: %w", err)
	}
	runningAfterResume := containsPid(running, pid)
	fmt.Fprintf(w, "STOP_RESUME running_after_resume=%v\n", runningAfterResume)
	if !runningAfterResume {
		return fmt.Errorf("process %s is not running after resume", pid)
	}

	output, err := sendEcho(pid, stopResumePlain)
	if err != nil {
		return fmt.Errorf("send resumed echo message: %w", err)
	}
	if output["Plain"] != stopResumePlain {
		return fmt.Errorf("unexpected stop-resume echo output")
	}
	fmt.Fprintln(w, "STOP_RESUME resume_decrypted=true")
	return nil
}

func isStoppedSendError(err error) bool {
	if err == nil {
		return false
	}
	errText := err.Error()
	return strings.Contains(errText, "err_process_stopped")
}

func sendPlainEchoForStopResume(pid, plain string) (map[string]string, error) {
	msgRes, err := sendPlainEchoMessage(pid, plain)
	if err != nil {
		return nil, err
	}
	result, err := s.ResultAndWait(pid, msgRes.Id)
	if err != nil {
		return nil, err
	}
	by, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return verifyEchoMessage(string(by), echoExpectation{
		Secret: "",
		Plain:  plain,
	})
}

func sendPlainEchoMessage(pid, plain string) (*serverSchema.Response, error) {
	msg := hymxSchema.Message{
		Base: hymxSchema.DefaultBaseMessage,
	}
	msgTags, err := utils.MessageToTags(msg)
	if err != nil {
		return nil, err
	}
	msgTags = utils.MergeTags(msgTags, []goarSchema.Tag{{Name: "Plain", Value: plain}})
	item, err := s.Bundler.CreateAndSignItem([]byte{}, pid, "", msgTags)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(url, "/")+"/", bytes.NewReader(item.Binary))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, serverHTTPError(resp)
	}
	var result serverSchema.Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode send response: %w", err)
	}
	return &result, nil
}

func serverHTTPError(resp *http.Response) error {
	var serverErr serverSchema.RespErr
	if err := json.NewDecoder(resp.Body).Decode(&serverErr); err == nil && serverErr.Err != "" {
		return errors.New(serverErr.Err)
	}
	return fmt.Errorf("invalid server response: %d", resp.StatusCode)
}

func containsPid(pids []string, pid string) bool {
	for _, candidate := range pids {
		if candidate == pid {
			return true
		}
	}
	return false
}
