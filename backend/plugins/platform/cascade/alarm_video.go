package cascade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"io"
	"noyo/core/alarmmedia"
	"noyo/core/store"
	"noyo/core/types"
	"os"
	"sync"
	"time"
)

const alarmVideoChunkSize = 64 << 10

type alarmVideoFile struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type alarmVideoMessage struct {
	Request string                    `json:"request"`
	Op      string                    `json:"op"`
	Record  alarmmedia.Record         `json:"record"`
	Files   map[string]alarmVideoFile `json:"files,omitempty"`
	Kind    string                    `json:"kind,omitempty"`
	Offset  int64                     `json:"offset,omitempty"`
	Data    []byte                    `json:"data,omitempty"`
}
type alarmVideoAck struct {
	Request string `json:"request"`
	Offset  int64  `json:"offset"`
	Done    bool   `json:"done"`
	Error   string `json:"error,omitempty"`
}
type alarmVideoEnvelope struct {
	gateway string
	message alarmVideoMessage
}

var alarmVideoReceiveMu sync.Mutex

func gatewayOwnsAlarmDevice(gateway, code string) bool {
	gw, e := store.GetDevice(gateway)
	if e != nil || gw == nil || gw.TenantID == 0 || gw.ProjectID == 0 {
		return false
	}
	seen := map[string]bool{}
	for i := 0; i < 32 && code != ""; i++ {
		if seen[code] {
			return false
		}
		seen[code] = true
		d, e := store.GetDevice(code)
		if e != nil || d == nil || d.TenantID != gw.TenantID || d.ProjectID != gw.ProjectID {
			return false
		}
		if d.ParentCode == gateway {
			return true
		}
		code = d.ParentCode
	}
	return false
}

// Called only from authenticated cascade telemetry after checking device ownership.
func registerRemoteAlarmVideo(gateway, device, id string) bool {
	if !alarmmedia.ValidID(id) || !gatewayOwnsAlarmDevice(gateway, device) {
		return false
	}
	old, e := alarmmedia.Load(id)
	if e == nil {
		return old.Remote && old.DeviceCode == device
	}
	if !os.IsNotExist(e) || !alarmmedia.HasCapacity() {
		return false
	}
	d, err := store.GetDevice(device)
	if err != nil || d == nil {
		return false
	}
	return alarmmedia.Save(alarmmedia.Record{ID: id, DeviceCode: device, TenantID: d.TenantID, ProjectID: d.ProjectID, Status: "uploading", Remote: true}) == nil
}

func (e *gatewayEngineImpl) alarmVideoUploadLoop(ctx context.Context) {
	alarmmedia.PruneOriginals()
	type retryState struct {
		next  time.Time
		delay time.Duration
	}
	retries := map[string]retryState{}
	timer := time.NewTicker(10 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if e.client == nil || !e.client.IsConnected() || !e.isRegistered.Load() {
			continue
		}
		for _, record := range alarmmedia.List() {
			if record.Remote || record.Uploaded || (record.Status != "ready" && record.Status != "partial" && record.Status != "failed") {
				continue
			}
			if time.Now().Before(retries[record.ID].next) {
				continue
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
			device, err := store.GetDevice(record.DeviceCode)
			if err != nil || device == nil {
				continue
			}
			mapped := e.prepareGatewayTelemetryEvent(e.gatewayCode, types.Event{Topic: record.DeviceCode}, device)
			localID := record.ID
			record.DeviceCode = mapped.Topic
			if err = e.uploadAlarmVideo(ctx, record); err == nil {
				_ = alarmmedia.Update(localID, func(r *alarmmedia.Record) { r.Uploaded = true })
				delete(retries, localID)
			} else {
				retry := retries[localID]
				if retry.delay == 0 {
					retry.delay = 10 * time.Second
				} else {
					retry.delay *= 2
				}
				if retry.delay > 5*time.Minute {
					retry.delay = 5 * time.Minute
				}
				retry.next = time.Now().Add(retry.delay)
				retries[localID] = retry
			}
		}
	}
}
func (e *gatewayEngineImpl) sendAlarmVideo(ctx context.Context, m alarmVideoMessage) (alarmVideoAck, error) {
	m.Request = uuid.NewString()
	b, err := json.Marshal(m)
	if err != nil {
		return alarmVideoAck{}, err
	}
	token := e.client.Publish(fmt.Sprintf("noyo/cascade/gw/%s/alarm-video/up", e.gatewayCode), 1, false, b)
	select {
	case <-ctx.Done():
		return alarmVideoAck{}, ctx.Err()
	case <-token.Done():
		if token.Error() != nil {
			return alarmVideoAck{}, token.Error()
		}
	case <-time.After(10 * time.Second):
		return alarmVideoAck{}, errors.New("publish timeout")
	}
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return alarmVideoAck{}, ctx.Err()
		case <-timer.C:
			return alarmVideoAck{}, errors.New("upload acknowledgement timeout")
		case ack := <-e.alarmVideoAcks:
			if ack.Request != m.Request {
				continue
			}
			if ack.Error != "" {
				return ack, errors.New(ack.Error)
			}
			return ack, nil
		}
	}
}
func hashAlarmVideo(path string) (alarmVideoFile, error) {
	f, e := os.Open(path)
	if e != nil {
		return alarmVideoFile{}, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return alarmVideoFile{}, e
	}
	if st.Size() <= 0 || st.Size() > alarmmedia.MaxFileBytes {
		return alarmVideoFile{}, errors.New("file size limit")
	}
	h := sha256.New()
	n, e := io.Copy(h, io.LimitReader(f, alarmmedia.MaxFileBytes+1))
	return alarmVideoFile{Size: n, SHA256: hex.EncodeToString(h.Sum(nil))}, e
}
func (e *gatewayEngineImpl) uploadAlarmVideo(ctx context.Context, r alarmmedia.Record) error {
	files := map[string]alarmVideoFile{}
	if r.Status != "failed" {
		for _, kind := range []string{"preview.mp4"} {
			p, _ := alarmmedia.Path(r.ID, kind)
			info, err := hashAlarmVideo(p)
			if err != nil {
				return err
			}
			files[kind] = info
		}
	}
	ack, err := e.sendAlarmVideo(ctx, alarmVideoMessage{Op: "begin", Record: r, Files: files})
	if err != nil || ack.Done {
		return err
	}
	for _, kind := range []string{"preview.mp4"} {
		ack, err = e.sendAlarmVideo(ctx, alarmVideoMessage{Op: "offset", Record: r, Kind: kind})
		if err != nil {
			return err
		}
		p, _ := alarmmedia.Path(r.ID, kind)
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		offset := ack.Offset
		if offset < 0 || offset > files[kind].Size {
			f.Close()
			return errors.New("invalid resume offset")
		}
		_, err = f.Seek(offset, io.SeekStart)
		if err != nil {
			f.Close()
			return err
		}
		buf := make([]byte, alarmVideoChunkSize)
		for offset < files[kind].Size {
			n, readErr := f.Read(buf)
			if readErr != nil {
				f.Close()
				return readErr
			}
			ack, err = e.sendAlarmVideo(ctx, alarmVideoMessage{Op: "chunk", Record: r, Kind: kind, Offset: offset, Data: buf[:n]})
			if err != nil {
				f.Close()
				return err
			}
			if ack.Offset != offset+int64(n) {
				f.Close()
				return errors.New("unexpected upload offset")
			}
			offset = ack.Offset
		}
		f.Close()
	}
	_, err = e.sendAlarmVideo(ctx, alarmVideoMessage{Op: "complete", Record: r})
	return err
}
func (e *platformEngineImpl) handleAlarmVideo(_ mqtt.Client, msg mqtt.Message) {
	if msg.Retained() || len(msg.Payload()) > alarmVideoChunkSize*2 {
		return
	}
	ok, gw := parseTopicGwSn(msg.Topic(), "noyo/cascade/gw/%s/alarm-video/up")
	if !ok {
		return
	}
	var m alarmVideoMessage
	if json.Unmarshal(msg.Payload(), &m) != nil || !alarmmedia.ValidID(m.Record.ID) || len(m.Data) > alarmVideoChunkSize {
		return
	}
	select {
	case e.alarmVideoInbox <- alarmVideoEnvelope{gw, m}:
	default:
	} // Sender retries from durable acknowledged offset.
}
func (e *platformEngineImpl) alarmVideoReceiveLoop(ctx context.Context) {
	alarmmedia.PruneOriginals()
	for {
		select {
		case <-ctx.Done():
			return
		case env := <-e.alarmVideoInbox:
			ack := alarmVideoAck{Request: env.message.Request}
			if !gatewayOwnsAlarmDevice(env.gateway, env.message.Record.DeviceCode) {
				ack.Error = "device_forbidden"
			} else {
				ack.Offset, ack.Done = 0, false
				var err error
				ack.Offset, ack.Done, err = receiveAlarmVideo(env.message)
				if err != nil {
					ack.Error = err.Error()
				}
			}
			b, _ := json.Marshal(ack)
			e.client.Publish(fmt.Sprintf("noyo/cascade/gw/%s/alarm-video/down", env.gateway), 1, false, b)
		}
	}
}

// Single bounded worker plus this lock serializes manifest and file mutation.
// Restart recovery uses file length; final SHA-256 catches any interrupted write.
func receiveAlarmVideo(m alarmVideoMessage) (int64, bool, error) {
	alarmVideoReceiveMu.Lock()
	defer alarmVideoReceiveMu.Unlock()
	old, err := alarmmedia.Load(m.Record.ID)
	if err != nil {
		return 0, false, errors.New("alarm_not_received")
	}
	if !old.Remote || old.DeviceCode != m.Record.DeviceCode {
		return 0, false, errors.New("recording_forbidden")
	}
	if old.Status == "ready" || old.Status == "partial" || old.Status == "failed" {
		return 0, true, nil
	}
	metaPath, _ := alarmmedia.Path(m.Record.ID, "upload.json")
	if m.Op == "begin" {
		if m.Record.Status != "ready" && m.Record.Status != "partial" && m.Record.Status != "failed" {
			return 0, false, errors.New("invalid_state")
		}
		if m.Record.Status == "failed" {
			m.Record.Remote = true
			m.Record.Uploaded = true
			return 0, true, alarmmedia.Save(m.Record)
		}
		if len(m.Files) != 1 {
			return 0, false, errors.New("invalid_files")
		}
		for _, kind := range []string{"preview.mp4"} {
			info := m.Files[kind]
			if info.Size <= 0 || info.Size > alarmmedia.MaxFileBytes || len(info.SHA256) != 64 {
				return 0, false, errors.New("file_limit")
			}
			if _, e := hex.DecodeString(info.SHA256); e != nil {
				return 0, false, e
			}
		}
		if previous, e := os.ReadFile(metaPath); e == nil {
			var saved alarmVideoMessage
			if json.Unmarshal(previous, &saved) != nil || saved.Files["preview.mp4"] != m.Files["preview.mp4"] {
				return 0, false, errors.New("manifest_conflict")
			}
			return 0, false, nil
		}
		if !alarmmedia.HasCapacity() {
			return 0, false, errors.New("storage_limit")
		}
		b, _ := json.Marshal(m)
		if err := os.WriteFile(metaPath+".tmp", b, 0600); err != nil {
			return 0, false, err
		}
		return 0, false, os.Rename(metaPath+".tmp", metaPath)
	}
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		return 0, false, err
	}
	var manifest alarmVideoMessage
	if err = json.Unmarshal(raw, &manifest); err != nil {
		return 0, false, err
	}
	if m.Op == "complete" {
		for _, kind := range []string{"preview.mp4"} {
			part, _ := alarmmedia.Path(m.Record.ID, kind+".part")
			dest, _ := alarmmedia.Path(m.Record.ID, kind)
			if _, err = os.Stat(part); os.IsNotExist(err) {
				part = dest
			}
			info, e := hashAlarmVideo(part)
			if e != nil || info != manifest.Files[kind] {
				_ = os.Remove(part)
				return 0, false, errors.New("checksum_mismatch")
			}
			if part != dest {
				if err = os.Rename(part, dest); err != nil {
					return 0, false, err
				}
			}
		}
		r := manifest.Record
		r.Remote = true
		r.Uploaded = true
		return 0, true, alarmmedia.Save(r)
	}
	if m.Kind != "preview.mp4" {
		return 0, false, errors.New("invalid_kind")
	}
	p, _ := alarmmedia.Path(m.Record.ID, m.Kind+".part")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return 0, false, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return 0, false, err
	}
	if m.Op == "offset" {
		return st.Size(), false, nil
	}
	if m.Op != "chunk" || m.Offset != st.Size() || len(m.Data) == 0 || len(m.Data) > alarmVideoChunkSize || m.Offset+int64(len(m.Data)) > manifest.Files[m.Kind].Size {
		return st.Size(), false, errors.New("invalid_chunk")
	}
	n, err := f.WriteAt(m.Data, m.Offset)
	if err != nil {
		return 0, false, err
	}
	if err = f.Sync(); err != nil {
		return 0, false, err
	}
	return m.Offset + int64(n), false, nil
}
