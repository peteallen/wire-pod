package sdkapp

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fforchino/vector-go-sdk/pkg/vectorpb"
)

const maxActivityEvents = 12

var activityMu sync.Mutex

type ActivityMonitor struct {
	StreamActive     bool                `json:"stream_active"`
	StartedAt        time.Time           `json:"started_at,omitempty"`
	LastEventAt      time.Time           `json:"last_event_at,omitempty"`
	LastError        string              `json:"last_error,omitempty"`
	WakeWord         *ActivityWakeWord   `json:"wake_word,omitempty"`
	UserIntent       *ActivityIntent     `json:"user_intent,omitempty"`
	Stimulation      *ActivityStim       `json:"stimulation,omitempty"`
	RobotState       *ActivityRobotState `json:"robot_state,omitempty"`
	Events           []ActivityEvent     `json:"events"`
	lastRobotLabel   string
	lastRobotMoving  bool
	lastRobotTouched bool
	lastRobotObject  bool
}

type ActivitySnapshot struct {
	Status         string              `json:"status"`
	StreamActive   bool                `json:"stream_active"`
	StartedAt      string              `json:"started_at,omitempty"`
	LastEventAt    string              `json:"last_event_at,omitempty"`
	LastEventAgeMs int64               `json:"last_event_age_ms,omitempty"`
	LastError      string              `json:"last_error,omitempty"`
	WakeWord       *ActivityWakeWord   `json:"wake_word,omitempty"`
	UserIntent     *ActivityIntent     `json:"user_intent,omitempty"`
	Stimulation    *ActivityStim       `json:"stimulation,omitempty"`
	RobotState     *ActivityRobotState `json:"robot_state,omitempty"`
	Events         []ActivityEvent     `json:"events"`
}

type ActivityEvent struct {
	Type   string    `json:"type"`
	Label  string    `json:"label"`
	Detail string    `json:"detail"`
	At     time.Time `json:"at"`
}

type ActivityWakeWord struct {
	State       string    `json:"state"`
	IntentHeard bool      `json:"intent_heard"`
	IntentJSON  string    `json:"intent_json,omitempty"`
	At          time.Time `json:"at"`
}

type ActivityIntent struct {
	IntentID uint32    `json:"intent_id"`
	Label    string    `json:"label"`
	JSONData string    `json:"json_data,omitempty"`
	At       time.Time `json:"at"`
}

type ActivityStim struct {
	Value         float32   `json:"value"`
	Velocity      float32   `json:"velocity"`
	Accel         float32   `json:"accel"`
	EmotionEvents []string  `json:"emotion_events,omitempty"`
	At            time.Time `json:"at"`
}

type ActivityRobotState struct {
	Label             string    `json:"label"`
	StatusRaw         uint32    `json:"status_raw"`
	StatusFlags       []string  `json:"status_flags"`
	Moving            bool      `json:"moving"`
	WheelSpeedMmps    float32   `json:"wheel_speed_mmps"`
	LeftWheelMmps     float32   `json:"left_wheel_mmps"`
	RightWheelMmps    float32   `json:"right_wheel_mmps"`
	HeadAngleDeg      float32   `json:"head_angle_deg"`
	LiftHeightMm      float32   `json:"lift_height_mm"`
	BeingTouched      bool      `json:"being_touched"`
	RawTouchValue     uint32    `json:"raw_touch_value"`
	ProximityMm       uint32    `json:"proximity_mm"`
	ProximityDetected bool      `json:"proximity_detected"`
	At                time.Time `json:"at"`
}

func newActivityMonitor() *ActivityMonitor {
	return &ActivityMonitor{
		Events: []ActivityEvent{},
	}
}

func ensureActivityMonitor(robotIndex int) *ActivityMonitor {
	if robotIndex < 0 || robotIndex >= len(robots) {
		return newActivityMonitor()
	}
	if robots[robotIndex].Activity == nil {
		robots[robotIndex].Activity = newActivityMonitor()
	}
	return robots[robotIndex].Activity
}

func startActivityStream(robotObj Robot, robotIndex int) error {
	activityMu.Lock()
	monitor := ensureActivityMonitor(robotIndex)
	if monitor.StreamActive {
		activityMu.Unlock()
		return nil
	}
	monitor.StreamActive = true
	monitor.StartedAt = time.Now()
	monitor.LastError = ""
	activityMu.Unlock()

	client, err := robotObj.Vector.Conn.EventStream(
		robotObj.Ctx,
		&vectorpb.EventRequest{
			ListType: &vectorpb.EventRequest_WhiteList{
				WhiteList: &vectorpb.FilterList{
					List: []string{"robot_state", "wake_word", "stimulation_info", "user_intent"},
				},
			},
			ConnectionId: "wirepod-activity",
		},
	)
	if err != nil {
		recordActivityError(robotIndex, err)
		return err
	}

	go func() {
		defer client.CloseSend()
		for activityStreamActive(robotIndex) {
			resp, err := client.Recv()
			if err != nil {
				recordActivityError(robotIndex, err)
				return
			}
			if resp != nil && resp.Event != nil {
				recordActivityEvent(robotIndex, resp.Event)
			}
		}
	}()

	return nil
}

func stopActivityStream(robotIndex int) {
	activityMu.Lock()
	defer activityMu.Unlock()
	if robotIndex < 0 || robotIndex >= len(robots) || robots[robotIndex].Activity == nil {
		return
	}
	robots[robotIndex].Activity.StreamActive = false
}

func activityStreamActive(robotIndex int) bool {
	activityMu.Lock()
	defer activityMu.Unlock()
	if robotIndex < 0 || robotIndex >= len(robots) || robots[robotIndex].Activity == nil {
		return false
	}
	return robots[robotIndex].Activity.StreamActive
}

func recordActivityError(robotIndex int, err error) {
	activityMu.Lock()
	defer activityMu.Unlock()
	if robotIndex < 0 || robotIndex >= len(robots) {
		return
	}
	monitor := ensureActivityMonitor(robotIndex)
	monitor.StreamActive = false
	monitor.LastError = err.Error()
	appendActivityEvent(monitor, ActivityEvent{
		Type:   "stream",
		Label:  "Activity stream stopped",
		Detail: err.Error(),
		At:     time.Now(),
	})
}

func recordActivityEvent(robotIndex int, event *vectorpb.Event) {
	now := time.Now()
	activityMu.Lock()
	defer activityMu.Unlock()
	if robotIndex < 0 || robotIndex >= len(robots) {
		return
	}
	monitor := ensureActivityMonitor(robotIndex)
	monitor.LastEventAt = now
	monitor.LastError = ""

	switch event.EventType.(type) {
	case *vectorpb.Event_WakeWord:
		recordWakeWord(monitor, event.GetWakeWord(), now)
	case *vectorpb.Event_UserIntent:
		recordUserIntent(monitor, event.GetUserIntent(), now)
	case *vectorpb.Event_StimulationInfo:
		recordStimulation(monitor, event.GetStimulationInfo(), now)
	case *vectorpb.Event_RobotState:
		recordRobotState(monitor, event.GetRobotState(), now)
	}
}

func recordWakeWord(monitor *ActivityMonitor, wake *vectorpb.WakeWord, now time.Time) {
	if wake == nil {
		return
	}
	state := "listening"
	detail := "Vector heard the wake word and started listening."
	intentHeard := false
	intentJSON := ""
	if end := wake.GetWakeWordEnd(); end != nil {
		state = "complete"
		intentHeard = end.GetIntentHeard()
		intentJSON = end.GetIntentJson()
		if intentHeard {
			detail = "Wake word session ended with intent data."
		} else {
			detail = "Wake word session ended without an intent."
		}
	}
	monitor.WakeWord = &ActivityWakeWord{
		State:       state,
		IntentHeard: intentHeard,
		IntentJSON:  intentJSON,
		At:          now,
	}
	appendActivityEvent(monitor, ActivityEvent{
		Type:   "wake_word",
		Label:  "Wake word",
		Detail: detail,
		At:     now,
	})
}

func recordUserIntent(monitor *ActivityMonitor, intent *vectorpb.UserIntent, now time.Time) {
	if intent == nil {
		return
	}
	label := extractIntentLabel(intent.GetJsonData())
	if label == "" {
		label = fmt.Sprintf("Intent %d", intent.GetIntentId())
	}
	monitor.UserIntent = &ActivityIntent{
		IntentID: intent.GetIntentId(),
		Label:    label,
		JSONData: intent.GetJsonData(),
		At:       now,
	}
	appendActivityEvent(monitor, ActivityEvent{
		Type:   "user_intent",
		Label:  "Intent",
		Detail: label,
		At:     now,
	})
}

func recordStimulation(monitor *ActivityMonitor, stim *vectorpb.StimulationInfo, now time.Time) {
	if stim == nil {
		return
	}
	monitor.Stimulation = &ActivityStim{
		Value:         stim.GetValue(),
		Velocity:      stim.GetVelocity(),
		Accel:         stim.GetAccel(),
		EmotionEvents: stim.GetEmotionEvents(),
		At:            now,
	}
	if len(stim.GetEmotionEvents()) > 0 {
		appendActivityEvent(monitor, ActivityEvent{
			Type:   "stimulation",
			Label:  "Emotion",
			Detail: strings.Join(stim.GetEmotionEvents(), ", "),
			At:     now,
		})
	}
}

func recordRobotState(monitor *ActivityMonitor, state *vectorpb.RobotState, now time.Time) {
	if state == nil {
		return
	}
	left := state.GetLeftWheelSpeedMmps()
	right := state.GetRightWheelSpeedMmps()
	wheelSpeed := float32(math.Max(math.Abs(float64(left)), math.Abs(float64(right))))
	touch := state.GetTouchData()
	prox := state.GetProxData()
	statusRaw := state.GetStatus()
	flags := robotStatusFlags(statusRaw)
	moving := wheelSpeed > 2 || hasStatus(statusRaw, vectorpb.RobotStatus_ROBOT_STATUS_IS_MOVING) || hasStatus(statusRaw, vectorpb.RobotStatus_ROBOT_STATUS_ARE_WHEELS_MOVING) || hasStatus(statusRaw, vectorpb.RobotStatus_ROBOT_STATUS_IS_PATHING)
	touched := touch.GetIsBeingTouched()
	objectDetected := prox.GetFoundObject()
	label := robotStateLabel(statusRaw, moving, touched, objectDetected)
	monitor.RobotState = &ActivityRobotState{
		Label:             label,
		StatusRaw:         statusRaw,
		StatusFlags:       flags,
		Moving:            moving,
		WheelSpeedMmps:    wheelSpeed,
		LeftWheelMmps:     left,
		RightWheelMmps:    right,
		HeadAngleDeg:      radiansToDegrees(state.GetHeadAngleRad()),
		LiftHeightMm:      state.GetLiftHeightMm(),
		BeingTouched:      touched,
		RawTouchValue:     touch.GetRawTouchValue(),
		ProximityMm:       prox.GetDistanceMm(),
		ProximityDetected: objectDetected,
		At:                now,
	}
	if label != monitor.lastRobotLabel || moving != monitor.lastRobotMoving || touched != monitor.lastRobotTouched || objectDetected != monitor.lastRobotObject {
		appendActivityEvent(monitor, ActivityEvent{
			Type:   "robot_state",
			Label:  "Robot state",
			Detail: label,
			At:     now,
		})
	}
	monitor.lastRobotLabel = label
	monitor.lastRobotMoving = moving
	monitor.lastRobotTouched = touched
	monitor.lastRobotObject = objectDetected
}

func getActivitySnapshot(robotIndex int) ActivitySnapshot {
	activityMu.Lock()
	defer activityMu.Unlock()
	monitor := ensureActivityMonitor(robotIndex)
	snapshot := ActivitySnapshot{
		Status:       activityStatus(monitor),
		StreamActive: monitor.StreamActive,
		LastError:    monitor.LastError,
		WakeWord:     monitor.WakeWord,
		UserIntent:   monitor.UserIntent,
		Stimulation:  monitor.Stimulation,
		RobotState:   monitor.RobotState,
		Events:       append([]ActivityEvent{}, monitor.Events...),
	}
	if !monitor.StartedAt.IsZero() {
		snapshot.StartedAt = monitor.StartedAt.Format(time.RFC3339Nano)
	}
	if !monitor.LastEventAt.IsZero() {
		snapshot.LastEventAt = monitor.LastEventAt.Format(time.RFC3339Nano)
		snapshot.LastEventAgeMs = time.Since(monitor.LastEventAt).Milliseconds()
	}
	return snapshot
}

func writeActivitySnapshot(w http.ResponseWriter, snapshot ActivitySnapshot) {
	w.Header().Set("Content-Type", "application/json")
	jsonBytes, err := json.Marshal(snapshot)
	if err != nil {
		http.Error(w, "error marshaling activity snapshot", http.StatusInternalServerError)
		return
	}
	w.Write(jsonBytes)
}

func streamLiveActivity(w http.ResponseWriter, r *http.Request, robotObj Robot, robotIndex int) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	client, err := robotObj.Vector.Conn.EventStream(
		r.Context(),
		&vectorpb.EventRequest{
			ListType: &vectorpb.EventRequest_WhiteList{
				WhiteList: &vectorpb.FilterList{
					List: []string{"robot_state", "wake_word", "stimulation_info", "user_intent"},
				},
			},
			ConnectionId: "wirepod-live-activity",
		},
	)
	if err != nil {
		recordActivityError(robotIndex, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.CloseSend()

	activityMu.Lock()
	monitor := ensureActivityMonitor(robotIndex)
	monitor.StreamActive = true
	monitor.StartedAt = time.Now()
	monitor.LastError = ""
	activityMu.Unlock()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	writeActivitySSE(w, flusher, getActivitySnapshot(robotIndex))

	for {
		resp, err := client.Recv()
		if err != nil {
			if r.Context().Err() != nil {
				stopActivityStream(robotIndex)
				return
			}
			recordActivityError(robotIndex, err)
			writeActivitySSE(w, flusher, getActivitySnapshot(robotIndex))
			return
		}
		if resp == nil || resp.Event == nil {
			continue
		}
		recordActivityEvent(robotIndex, resp.Event)
		writeActivitySSE(w, flusher, getActivitySnapshot(robotIndex))
	}
}

func writeActivitySSE(w http.ResponseWriter, flusher http.Flusher, snapshot ActivitySnapshot) {
	jsonBytes, err := json.Marshal(snapshot)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: robot_activity\n")
	fmt.Fprintf(w, "data: %s\n\n", jsonBytes)
	flusher.Flush()
}

func appendActivityEvent(monitor *ActivityMonitor, event ActivityEvent) {
	monitor.Events = append(monitor.Events, event)
	if len(monitor.Events) > maxActivityEvents {
		monitor.Events = monitor.Events[len(monitor.Events)-maxActivityEvents:]
	}
}

func activityStatus(monitor *ActivityMonitor) string {
	if monitor.LastError != "" {
		return "error"
	}
	if monitor.StreamActive && monitor.LastEventAt.IsZero() {
		return "connecting"
	}
	if monitor.StreamActive {
		return "live"
	}
	return "stopped"
}

func robotStatusFlags(statusRaw uint32) []string {
	statuses := []struct {
		flag  vectorpb.RobotStatus
		label string
	}{
		{vectorpb.RobotStatus_ROBOT_STATUS_IS_MOVING, "Moving"},
		{vectorpb.RobotStatus_ROBOT_STATUS_ARE_WHEELS_MOVING, "Wheels moving"},
		{vectorpb.RobotStatus_ROBOT_STATUS_IS_PATHING, "Pathing"},
		{vectorpb.RobotStatus_ROBOT_STATUS_IS_ANIMATING, "Animating"},
		{vectorpb.RobotStatus_ROBOT_STATUS_IS_CARRYING_BLOCK, "Carrying cube"},
		{vectorpb.RobotStatus_ROBOT_STATUS_IS_PICKING_OR_PLACING, "Picking or placing"},
		{vectorpb.RobotStatus_ROBOT_STATUS_IS_PICKED_UP, "Picked up"},
		{vectorpb.RobotStatus_ROBOT_STATUS_IS_BEING_HELD, "Being held"},
		{vectorpb.RobotStatus_ROBOT_STATUS_IS_BUTTON_PRESSED, "Button pressed"},
		{vectorpb.RobotStatus_ROBOT_STATUS_IS_FALLING, "Falling"},
		{vectorpb.RobotStatus_ROBOT_STATUS_IS_ON_CHARGER, "On charger"},
		{vectorpb.RobotStatus_ROBOT_STATUS_IS_CHARGING, "Charging"},
		{vectorpb.RobotStatus_ROBOT_STATUS_CLIFF_DETECTED, "Cliff detected"},
		{vectorpb.RobotStatus_ROBOT_STATUS_IS_MOTION_DETECTED, "Motion detected"},
		{vectorpb.RobotStatus_ROBOT_STATUS_CALM_POWER_MODE, "Calm power mode"},
	}
	flags := []string{}
	for _, status := range statuses {
		if hasStatus(statusRaw, status.flag) {
			flags = append(flags, status.label)
		}
	}
	return flags
}

func robotStateLabel(statusRaw uint32, moving, touched, objectDetected bool) string {
	switch {
	case hasStatus(statusRaw, vectorpb.RobotStatus_ROBOT_STATUS_IS_FALLING):
		return "Falling"
	case hasStatus(statusRaw, vectorpb.RobotStatus_ROBOT_STATUS_IS_PICKED_UP), hasStatus(statusRaw, vectorpb.RobotStatus_ROBOT_STATUS_IS_BEING_HELD):
		return "Being held"
	case touched:
		return "Being touched"
	case hasStatus(statusRaw, vectorpb.RobotStatus_ROBOT_STATUS_IS_ANIMATING):
		return "Animating"
	case moving:
		return "Moving"
	case hasStatus(statusRaw, vectorpb.RobotStatus_ROBOT_STATUS_IS_ON_CHARGER):
		if hasStatus(statusRaw, vectorpb.RobotStatus_ROBOT_STATUS_IS_CHARGING) {
			return "Docked and charging"
		}
		return "Docked"
	case objectDetected:
		return "Object detected"
	default:
		return "Idle"
	}
}

func hasStatus(statusRaw uint32, flag vectorpb.RobotStatus) bool {
	return statusRaw&uint32(flag) > 0
}

func radiansToDegrees(value float32) float32 {
	return value * 180 / math.Pi
}

func extractIntentLabel(raw string) string {
	if raw == "" {
		return ""
	}
	var data interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return summarizeRawIntent(raw)
	}
	if found := findIntentString(data); found != "" {
		return found
	}
	return summarizeRawIntent(raw)
}

func findIntentString(value interface{}) string {
	switch typed := value.(type) {
	case map[string]interface{}:
		for _, key := range []string{"intent", "intent_name", "intentName", "name", "query", "utterance"} {
			if raw, ok := typed[key].(string); ok && strings.TrimSpace(raw) != "" {
				return strings.TrimSpace(raw)
			}
		}
		for _, nested := range typed {
			if found := findIntentString(nested); found != "" {
				return found
			}
		}
	case []interface{}:
		for _, nested := range typed {
			if found := findIntentString(nested); found != "" {
				return found
			}
		}
	case string:
		return strings.TrimSpace(typed)
	}
	return ""
}

func summarizeRawIntent(raw string) string {
	cleaned := strings.Join(strings.Fields(raw), " ")
	if len(cleaned) > 96 {
		return cleaned[:95] + "..."
	}
	return cleaned
}
