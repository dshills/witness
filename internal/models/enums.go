package models

import (
	"encoding/json"
	"fmt"
)

// RunStatus represents the lifecycle state of a run.
type RunStatus int

const (
	RunStatusPending RunStatus = iota
	RunStatusRunning
	RunStatusCompleted
	RunStatusFailed
	RunStatusCancelled
	RunStatusStalled
	RunStatusUnknown
)

var runStatusNames = [...]string{
	"pending",
	"running",
	"completed",
	"failed",
	"cancelled",
	"stalled",
	"unknown",
}

var runStatusMap = func() map[string]RunStatus {
	m := make(map[string]RunStatus, len(runStatusNames))
	for i, name := range runStatusNames {
		m[name] = RunStatus(i)
	}
	return m
}()

func (s RunStatus) String() string {
	if int(s) < len(runStatusNames) {
		return runStatusNames[s]
	}
	return "unknown"
}

func (s RunStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *RunStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	if v, ok := runStatusMap[str]; ok {
		*s = v
		return nil
	}
	return fmt.Errorf("unknown RunStatus: %q", str)
}

// StageStatus represents the lifecycle state of a stage.
type StageStatus int

const (
	StageStatusPending StageStatus = iota
	StageStatusRunning
	StageStatusCompleted
	StageStatusFailed
	StageStatusSkipped
)

var stageStatusNames = [...]string{
	"pending",
	"running",
	"completed",
	"failed",
	"skipped",
}

var stageStatusMap = func() map[string]StageStatus {
	m := make(map[string]StageStatus, len(stageStatusNames))
	for i, name := range stageStatusNames {
		m[name] = StageStatus(i)
	}
	return m
}()

func (s StageStatus) String() string {
	if int(s) < len(stageStatusNames) {
		return stageStatusNames[s]
	}
	return "unknown"
}

func (s StageStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *StageStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	if v, ok := stageStatusMap[str]; ok {
		*s = v
		return nil
	}
	return fmt.Errorf("unknown StageStatus: %q", str)
}

// ChangeType represents the type of a file change.
type ChangeType int

const (
	ChangeTypeCreated ChangeType = iota
	ChangeTypeModified
	ChangeTypeDeleted
	ChangeTypeRenamed
)

var changeTypeNames = [...]string{
	"created",
	"modified",
	"deleted",
	"renamed",
}

var changeTypeMap = func() map[string]ChangeType {
	m := make(map[string]ChangeType, len(changeTypeNames))
	for i, name := range changeTypeNames {
		m[name] = ChangeType(i)
	}
	return m
}()

func (c ChangeType) String() string {
	if int(c) < len(changeTypeNames) {
		return changeTypeNames[c]
	}
	return "unknown"
}

func (c ChangeType) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

func (c *ChangeType) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	if v, ok := changeTypeMap[str]; ok {
		*c = v
		return nil
	}
	return fmt.Errorf("unknown ChangeType: %q", str)
}

// Severity represents alert severity levels.
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityError
	SeverityCritical
)

var severityNames = [...]string{
	"info",
	"warning",
	"error",
	"critical",
}

var severityMap = func() map[string]Severity {
	m := make(map[string]Severity, len(severityNames))
	for i, name := range severityNames {
		m[name] = Severity(i)
	}
	return m
}()

func (s Severity) String() string {
	if int(s) < len(severityNames) {
		return severityNames[s]
	}
	return "unknown"
}

func (s Severity) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *Severity) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	if v, ok := severityMap[str]; ok {
		*s = v
		return nil
	}
	return fmt.Errorf("unknown Severity: %q", str)
}

// InvocationStatus represents the status of a tool or model invocation.
type InvocationStatus int

const (
	InvocationStatusPending InvocationStatus = iota
	InvocationStatusRunning
	InvocationStatusCompleted
	InvocationStatusFailed
)

var invocationStatusNames = [...]string{
	"pending",
	"running",
	"completed",
	"failed",
}

var invocationStatusMap = func() map[string]InvocationStatus {
	m := make(map[string]InvocationStatus, len(invocationStatusNames))
	for i, name := range invocationStatusNames {
		m[name] = InvocationStatus(i)
	}
	return m
}()

func (s InvocationStatus) String() string {
	if int(s) < len(invocationStatusNames) {
		return invocationStatusNames[s]
	}
	return "unknown"
}

func (s InvocationStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *InvocationStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	if v, ok := invocationStatusMap[str]; ok {
		*s = v
		return nil
	}
	return fmt.Errorf("unknown InvocationStatus: %q", str)
}
