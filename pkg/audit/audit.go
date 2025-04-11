package audit

import (
	"fmt"
	"sync"
	"time"
)

// Package audit_log provides a plugin for logging device registration and authentication events.

type LogType string

const (
	LOG_MANAGER = "LogManager"

	TYPE_NORMAL   LogType = "normal"
	TYPE_UPDATE   LogType = "updates"
	TYPE_INCIDENT LogType = "incidents"
)

type LogManager interface {
	GetAuditLogs(typ string) ([]map[string]interface{}, error)
	AddLog(remoteAddr, serialNumber, desc string, code int, detail ...interface{})
	AddUpdateLog(remoteAddr, serialNumber, desc string, code int, detail ...interface{})
	AddIncidentLog(remoteAddr, serialNumber, desc string, code int, detail ...interface{})
}

type LogManagerImpl struct {
	mu   sync.Mutex
	Logs map[LogType][]map[string]interface{}
}

func NewManager() *LogManagerImpl {
	return &LogManagerImpl{
		Logs: make(map[LogType][]map[string]interface{}),
	}
}

func (l *LogManagerImpl) GetAuditLogs(typ string) ([]map[string]interface{}, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if logs, ok := l.Logs[LogType(typ)]; ok {
		return logs, nil
	}
	return nil, fmt.Errorf("no logs found for type: %s", typ)
}

func (l *LogManagerImpl) addLog(typ LogType, remoteAddr, serialNumber, desc string, code int, detail ...interface{}) {
	log := map[string]interface{}{
		"code":          code,
		"remote_addr":   remoteAddr,
		"serial_number": serialNumber,
		"description":   desc,
		"timestamp":     time.Now().Format(time.RFC3339),
	}
	if len(detail) > 0 {
		log["detail"] = detail[0]
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.Logs[typ]; !ok {
		l.Logs[typ] = make([]map[string]interface{}, 0)
	}
	l.Logs[typ] = append(l.Logs[typ], log)
}

func (l *LogManagerImpl) AddLog(remoteAddr, serialNumber, desc string, code int, detail ...interface{}) {
	l.addLog(TYPE_NORMAL, remoteAddr, serialNumber, desc, code, detail...)
}

func (l *LogManagerImpl) AddUpdateLog(remoteAddr, serialNumber, desc string, code int, detail ...interface{}) {
	l.addLog(TYPE_UPDATE, remoteAddr, serialNumber, desc, code, detail...)
}

func (l *LogManagerImpl) AddIncidentLog(remoteAddr, serialNumber, desc string, code int, detail ...interface{}) {
	l.addLog(TYPE_INCIDENT, remoteAddr, serialNumber, desc, code, detail...)
}
