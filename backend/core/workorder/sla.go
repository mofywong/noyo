package workorder

import (
	"context"
	"sort"
	"strings"
	"time"
)

type BusinessCalendar struct {
	Location           *time.Location
	Weekdays           map[time.Weekday]bool
	StartHour, EndHour int
	Holidays           map[string]bool
}

func DefaultBusinessCalendar() BusinessCalendar {
	return BusinessCalendar{Location: time.UTC, Weekdays: map[time.Weekday]bool{time.Monday: true, time.Tuesday: true, time.Wednesday: true, time.Thursday: true, time.Friday: true}, StartHour: 9, EndHour: 18, Holidays: map[string]bool{}}
}
func (c BusinessCalendar) working(t time.Time) bool {
	if c.Location == nil {
		c.Location = time.UTC
	}
	local := t.In(c.Location)
	return c.Weekdays[local.Weekday()] && !c.Holidays[local.Format("2006-01-02")] && local.Hour() >= c.StartHour && local.Hour() < c.EndHour
}
func (c BusinessCalendar) AddWorking(start time.Time, duration time.Duration) time.Time {
	if duration <= 0 {
		return start
	}
	if c.EndHour <= c.StartHour {
		// Invalid calendar definitions must not spin forever.  Callers that
		// require a non-default calendar should reject it at publication time.
		return start
	}
	if c.Location == nil {
		c.Location = time.UTC
	}
	current := start
	remaining := duration
	for remaining > 0 {
		if !c.working(current) {
			current = current.Add(time.Minute)
			continue
		}
		end := time.Date(current.In(c.Location).Year(), current.In(c.Location).Month(), current.In(c.Location).Day(), c.EndHour, 0, 0, 0, c.Location)
		available := end.Sub(current)
		if available >= remaining {
			return current.Add(remaining)
		}
		remaining -= available
		current = end.Add(time.Minute)
	}
	return current
}

type SLAPolicy struct {
	ResponseAfter time.Duration
	ResolveAfter  time.Duration
	Calendar      BusinessCalendar
	WarningBefore time.Duration
	PauseReasons  []string
}
type SLAState struct {
	StartedAt, PausedAt, DueAt              *time.Time
	PausedDuration                          time.Duration
	Breached, WarningEmitted, BreachEmitted bool
}

func CalculateDueAt(start time.Time, policy SLAPolicy) time.Time {
	return policy.Calendar.AddWorking(start, policy.ResolveAfter)
}
func (s *SLAState) Pause(now time.Time) {
	if s.PausedAt == nil {
		s.PausedAt = &now
	}
}
func (s *SLAState) Resume(now time.Time) {
	if s.PausedAt != nil {
		s.PausedDuration += now.Sub(*s.PausedAt)
		s.PausedAt = nil
	}
}
func (s *SLAState) Events(now time.Time, warningBefore time.Duration) []string {
	events := []string{}
	if s.DueAt == nil {
		return events
	}
	if !s.WarningEmitted && now.Before(*s.DueAt) && !now.Before(s.DueAt.Add(-warningBefore)) {
		s.WarningEmitted = true
		events = append(events, "sla.warning")
	}
	if !s.BreachEmitted && !now.Before(*s.DueAt) {
		s.BreachEmitted = true
		s.Breached = true
		events = append(events, "sla.breached")
	}
	sort.Strings(events)
	return events
}

type SLAInstance struct {
	ID, WorkOrderPublicID                     string
	Scope                                     Scope
	StartedAt, ResponseDueAt, ResolutionDueAt time.Time
	State                                     SLAState
	Policy                                    SLAPolicy
	ResponseEventSent, ResolutionEventSent    bool
}
type SLAEvent struct {
	ID, Type, WorkOrderPublicID string
	Scope                       Scope
	At                          time.Time
}

const (
	SLAEventStarted          = "sla.started"
	SLAEventPaused           = "sla.paused"
	SLAEventResumed          = "sla.resumed"
	SLAEventWarning          = "sla.warning"
	SLAEventBreach           = "sla.breached"
	SLAEventResponseBreach   = "sla.response_breached"
	SLAEventResolutionBreach = "sla.resolution_breached"
	SLAEventCompleted        = "sla.completed"
	SLAEventReopened         = "sla.reopened"
)

type SLAService struct {
	calendar  BusinessCalendar
	instances map[string]*SLAInstance
	events    map[string]SLAEvent
}

func NewSLAService(calendars ...BusinessCalendar) *SLAService {
	calendar := DefaultBusinessCalendar()
	if len(calendars) > 0 {
		calendar = calendars[0]
	}
	if calendar.Location == nil {
		calendar.Location = time.UTC
	}
	if calendar.Weekdays == nil {
		calendar.Weekdays = DefaultBusinessCalendar().Weekdays
	}
	if calendar.Holidays == nil {
		calendar.Holidays = map[string]bool{}
	}
	return &SLAService{calendar: calendar, instances: map[string]*SLAInstance{}, events: map[string]SLAEvent{}}
}
func (s *SLAService) Start(_ context.Context, scope Scope, workOrderID string, policy SLAPolicy, start time.Time) (*SLAInstance, error) {
	if scope.TenantID == 0 || scope.ProjectID == 0 || workOrderID == "" {
		return nil, NewError(CodeValidationFailed, "SLA scope and work order are required")
	}
	if policy.Calendar.Location == nil {
		policy.Calendar = s.calendar
	}
	if policy.Calendar.EndHour <= policy.Calendar.StartHour {
		policy.Calendar.StartHour, policy.Calendar.EndHour = s.calendar.StartHour, s.calendar.EndHour
		if policy.Calendar.EndHour <= policy.Calendar.StartHour {
			defaultCalendar := DefaultBusinessCalendar()
			policy.Calendar.StartHour, policy.Calendar.EndHour = defaultCalendar.StartHour, defaultCalendar.EndHour
		}
	}
	if policy.Calendar.Holidays == nil {
		policy.Calendar.Holidays = s.calendar.Holidays
	}
	if policy.ResponseAfter < 0 || policy.ResolveAfter < 0 {
		return nil, NewError(CodeValidationFailed, "SLA durations cannot be negative")
	}
	start = start.UTC()
	due := CalculateDueAt(start, policy)
	instance := &SLAInstance{ID: workOrderID, WorkOrderPublicID: workOrderID, Scope: scope, StartedAt: start, ResolutionDueAt: due, Policy: policy, State: SLAState{StartedAt: &start, DueAt: &due}}
	if policy.ResponseAfter > 0 {
		instance.ResponseDueAt = policy.Calendar.AddWorking(start, policy.ResponseAfter)
	}
	s.emit(instance, SLAEventStarted, start)
	s.instances[scopeKey(scope, "sla:"+workOrderID)] = instance
	return instance, nil
}
func (s *SLAService) Pause(_ context.Context, scope Scope, id, reason string, at time.Time) error {
	instance, err := s.get(scope, id)
	if err != nil {
		return err
	}
	if instance.State.PausedAt != nil {
		return NewError(CodeValidationFailed, "SLA instance is already paused")
	}
	if !allowedPauseReason(instance.Policy, reason) {
		return NewError(CodeValidationFailed, "SLA pause reason is not allowed")
	}
	instance.State.Pause(at.UTC())
	s.emit(instance, SLAEventPaused, at)
	return nil
}
func (s *SLAService) Resume(_ context.Context, scope Scope, id string, at time.Time) error {
	instance, err := s.get(scope, id)
	if err != nil {
		return err
	}
	if instance.State.PausedAt == nil {
		return NewError(CodeValidationFailed, "SLA instance is not paused")
	}
	at = at.UTC()
	if at.Before(*instance.State.PausedAt) {
		return NewError(CodeValidationFailed, "SLA resume time precedes pause time")
	}
	delta := at.Sub(*instance.State.PausedAt)
	instance.State.Resume(at)
	if !instance.ResponseDueAt.IsZero() {
		instance.ResponseDueAt = instance.ResponseDueAt.Add(delta)
	}
	if !instance.ResolutionDueAt.IsZero() {
		instance.ResolutionDueAt = instance.ResolutionDueAt.Add(delta)
		instance.State.DueAt = &instance.ResolutionDueAt
	}
	s.emit(instance, SLAEventResumed, at)
	return nil
}
func (s *SLAService) Scan(_ context.Context, scope Scope, id string, at time.Time) ([]SLAEvent, error) {
	instance, err := s.get(scope, id)
	if err != nil {
		return nil, err
	}
	if instance.State.PausedAt != nil {
		return nil, nil
	}
	at = at.UTC()
	names := instance.State.Events(at, instance.Policy.WarningBefore)
	events := make([]SLAEvent, 0, len(names))
	for _, name := range names {
		event := s.emit(instance, name, at)
		if name == SLAEventBreach {
			instance.ResponseEventSent = true
			instance.ResolutionEventSent = true
		}
		events = append(events, event)
	}
	if !instance.ResponseDueAt.IsZero() && !instance.ResponseEventSent && !at.Before(instance.ResponseDueAt) {
		instance.ResponseEventSent = true
		events = append(events, s.emit(instance, SLAEventResponseBreach, at))
	}
	return events, nil
}
func (s *SLAService) Reopen(_ context.Context, scope Scope, id string, at time.Time) error {
	instance, err := s.get(scope, id)
	if err != nil {
		return err
	}
	at = at.UTC()
	instance.State.BreachEmitted, instance.State.WarningEmitted = false, false
	instance.ResponseEventSent, instance.ResolutionEventSent = false, false
	instance.StartedAt = at
	instance.ResolutionDueAt = CalculateDueAt(at, instance.Policy)
	instance.State.DueAt = &instance.ResolutionDueAt
	s.emit(instance, SLAEventReopened, at)
	return nil
}
func (s *SLAService) Complete(_ context.Context, scope Scope, id string, at time.Time) error {
	instance, err := s.get(scope, id)
	if err != nil {
		return err
	}
	s.emit(instance, SLAEventCompleted, at.UTC())
	return nil
}
func (s *SLAService) Events(scope Scope, id string) ([]SLAEvent, error) {
	if _, err := s.get(scope, id); err != nil {
		return nil, err
	}
	result := []SLAEvent{}
	prefix := id + ":"
	for key, event := range s.events {
		if strings.HasPrefix(key, prefix) && event.Scope.TenantID == scope.TenantID && event.Scope.ProjectID == scope.ProjectID {
			result = append(result, event)
		}
	}
	return result, nil
}
func (s *SLAService) get(scope Scope, id string) (*SLAInstance, error) {
	if scope.TenantID == 0 || scope.ProjectID == 0 || strings.TrimSpace(id) == "" {
		return nil, NewError(CodeValidationFailed, "SLA scope and id are required")
	}
	instance, ok := s.instances[scopeKey(scope, "sla:"+id)]
	if !ok || instance.Scope.TenantID != scope.TenantID || instance.Scope.ProjectID != scope.ProjectID {
		return nil, NewError(CodeWorkOrderNotFound, "SLA instance not found")
	}
	return instance, nil
}
func (s *SLAService) emit(instance *SLAInstance, typ string, at time.Time) SLAEvent {
	id := instance.ID + ":" + typ
	if previous, ok := s.events[id]; ok {
		return previous
	}
	event := SLAEvent{ID: id, Type: typ, WorkOrderPublicID: instance.WorkOrderPublicID, Scope: instance.Scope, At: at.UTC()}
	s.events[id] = event
	return event
}
func allowedPauseReason(policy SLAPolicy, reason string) bool {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return false
	}
	if len(policy.PauseReasons) == 0 {
		return true
	}
	for _, allowed := range policy.PauseReasons {
		if strings.EqualFold(reason, strings.TrimSpace(allowed)) {
			return true
		}
	}
	return false
}
