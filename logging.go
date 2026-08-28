package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"
)

type ClientLogAttributes struct {
	ip string
	pk string
}
type TransactionLogAttributes struct {
	routine string
	tsid    string
}
type LogMsg struct {
	time        string
	level       string
	kind        string
	client      *ClientLogAttributes
	transaction *TransactionLogAttributes
	msg         string
}

type HarmonyHandler struct {
	stream     io.Writer
	streamLock *sync.Mutex
	attrs      []slog.Attr
	enabled    bool
}

func newHarmonyTextHandler(stream io.Writer, enabled bool) slog.Handler {
	return HarmonyHandler{
		stream:     stream,
		streamLock: &sync.Mutex{},
		enabled:    enabled,
	}
}

func (h HarmonyHandler) Enabled(context context.Context, level slog.Level) bool {
	return h.enabled
}

func (h HarmonyHandler) Handle(context context.Context, record slog.Record) error {

	log := LogMsg{}

	addAttr(&log, slog.Time(slog.TimeKey, record.Time))
	addAttr(&log, slog.Any(slog.LevelKey, record.Level))
	addAttr(&log, slog.String(slog.MessageKey, record.Message))

	// attrs added via Logger.With()
	for _, a := range h.attrs {
		addAttr(&log, a)
	}

	record.Attrs(func(a slog.Attr) bool {
		addAttr(&log, a)
		return true
	})

	buf := make([]byte, 0, 1024)

	buf = fmt.Appendf(buf, "[%s %s %-23s]", log.time, log.level, log.kind)
	if log.client != nil {
		buf = fmt.Appendf(buf, " %s %s", log.client.ip, getShortPkString(log.client.pk))
	}
	if log.transaction != nil {
		buf = fmt.Appendf(buf, " (%s %s)", log.transaction.tsid, log.transaction.routine)
	}
	buf = fmt.Appendf(buf, " %s\n", log.msg)

	defer h.streamLock.Unlock()
	h.streamLock.Lock()
	h.stream.Write(buf)

	return nil
}

func addAttr(log *LogMsg, a slog.Attr) {
	if a.Value.Kind() == slog.KindGroup {
		for _, a1 := range a.Value.Group() {
			addAttr(log, a1)
			return
		}
	}
	switch a.Key {
	case "time":
		log.time = a.Value.Time().Format(time.RFC3339)
	case "level":
		log.level = fmt.Sprintf("%s", a.Value)
	case "msg":
		log.msg = a.Value.String()
	case "kind":
		log.kind = a.Value.String()
	case "ip":
		if log.client == nil {
			log.client = &ClientLogAttributes{}
		}
		log.client.ip = a.Value.String()
	case "pk":
		if log.client == nil {
			log.client = &ClientLogAttributes{}
		}
		log.client.pk = a.Value.String()
	case "routine":
		if log.transaction == nil {
			log.transaction = &TransactionLogAttributes{}
		}
		log.transaction.routine = a.Value.String()
	case "tsid":
		if log.transaction == nil {
			log.transaction = &TransactionLogAttributes{}
		}
		log.transaction.tsid = a.Value.String()
	default:
	}
}

func (h HarmonyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return HarmonyHandler{
		stream:     h.stream,
		streamLock: h.streamLock,
		enabled:    h.enabled,
		attrs:      append(h.attrs, attrs...),
	}
}

func (h HarmonyHandler) WithGroup(name string) slog.Handler {
	return h
}

func getShortPkString(pk string) string {
	pkStr := string(pk)
	endIdx := min(32, len(pkStr))
	startIdx := max(0, endIdx-16)
	return pkStr[startIdx:endIdx]
}
