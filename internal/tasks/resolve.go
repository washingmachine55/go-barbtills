package tasks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"barbtils/internal/database"

	"github.com/google/uuid"
)

var (
	ErrTaskNotFound            = errors.New("task not found")
	ErrSessionNotFound         = errors.New("session not found")
	ErrNameTaken               = errors.New("a task with that name already exists")
	ErrNameEmpty               = errors.New("task name must not be empty")
	ErrAlreadyRunning          = errors.New("task already has an open session")
	ErrNotRunning              = errors.New("task has no open session")
	ErrRecurringCannotComplete = errors.New("recurring tasks cannot be completed; archive it instead")
)

// RefMode forces how a task reference is interpreted. RefAuto guesses, which is
// what every command does unless --name or --seq is given.
type RefMode uint8

const (
	RefAuto RefMode = iota
	RefName
	RefSeq
)

// ParseSeq accepts "3", "#3", "s3" and surrounding whitespace.
func ParseSeq(arg string) (int64, error) {
	t := strings.TrimSpace(arg)
	t = strings.TrimPrefix(t, "#")
	t = strings.TrimPrefix(t, "s")
	n, err := strconv.ParseInt(t, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid number %q: expected a positive integer like 3 or #3", arg)
	}
	return n, nil
}

// looksNumeric reports whether ref is nothing but digits, optionally behind a
// single '#'. A task named "42" is still reachable with --name.
func looksNumeric(ref string) bool {
	t := strings.TrimPrefix(strings.TrimSpace(ref), "#")
	if t == "" {
		return false
	}
	for _, c := range t {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ResolveTask is the only name/seq -> uuid translation point in the codebase.
//
// In RefAuto mode: all-digits (optionally "#3") means seq, a parseable UUID
// means id, anything else means a case-insensitive name. If a task is literally
// named with the same digits that are also a live seq, the reference is
// ambiguous and the caller is told to disambiguate with --name or --seq rather
// than the wrong task being acted on.
func (s *Store) ResolveTask(ctx context.Context, ref string, mode RefMode) (database.Task, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return database.Task{}, fmt.Errorf("%w: no task given", ErrTaskNotFound)
	}

	switch mode {
	case RefSeq:
		return s.taskBySeq(ctx, ref)
	case RefName:
		return s.taskByName(ctx, ref)
	}

	if id, err := uuid.Parse(ref); err == nil {
		t, err := s.q.GetTaskByID(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return database.Task{}, fmt.Errorf("%w: no task with id %s", ErrTaskNotFound, ref)
		}
		return t, err
	}

	if looksNumeric(ref) {
		bySeq, seqErr := s.taskBySeq(ctx, ref)
		byName, nameErr := s.taskByName(ctx, ref)
		switch {
		case seqErr == nil && nameErr == nil && bySeq.ID != byName.ID:
			return database.Task{}, fmt.Errorf(
				"%w: %q is both task #%d (%q) and the name of task #%d — use --seq or --name",
				ErrAmbiguousRef, ref, bySeq.Seq, bySeq.Name, byName.Seq)
		case seqErr == nil:
			return bySeq, nil
		case nameErr == nil:
			return byName, nil
		default:
			return database.Task{}, seqErr
		}
	}

	return s.taskByName(ctx, ref)
}

var ErrAmbiguousRef = errors.New("ambiguous task reference")

func (s *Store) taskBySeq(ctx context.Context, ref string) (database.Task, error) {
	n, err := ParseSeq(ref)
	if err != nil {
		return database.Task{}, err
	}
	t, err := s.q.GetTaskBySeq(ctx, n)
	if errors.Is(err, sql.ErrNoRows) {
		return database.Task{}, fmt.Errorf("%w: no task #%d", ErrTaskNotFound, n)
	}
	return t, err
}

func (s *Store) taskByName(ctx context.Context, name string) (database.Task, error) {
	t, err := s.q.GetTaskByName(ctx, name)
	if errors.Is(err, sql.ErrNoRows) {
		return database.Task{}, fmt.Errorf("%w: no task named %q", ErrTaskNotFound, name)
	}
	return t, err
}

// ResolveSession looks up one session by its own short number.
func (s *Store) ResolveSession(ctx context.Context, ref string) (database.TasksSession, error) {
	n, err := ParseSeq(ref)
	if err != nil {
		return database.TasksSession{}, err
	}
	sess, err := s.q.GetSessionBySeq(ctx, n)
	if errors.Is(err, sql.ErrNoRows) {
		return database.TasksSession{}, fmt.Errorf("%w: no session s%d", ErrSessionNotFound, n)
	}
	return sess, err
}
