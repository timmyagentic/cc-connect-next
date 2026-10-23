package core

import (
	"context"
	"errors"
	"time"
)

var errUpdateCheckInProgress = errors.New("update check already in progress")

type updateCheck struct {
	sessionKey string
	userID     string
	cancel     context.CancelFunc
	ctx        context.Context
}

func (e *Engine) beginUpdateCheck(msg *Message) (context.Context, string, func(), error) {
	token, err := newPendingActionToken()
	if err != nil {
		return nil, "", nil, err
	}
	ctx, cancel := context.WithTimeout(e.ctx, 45*time.Second)
	e.updateServiceMu.Lock()
	if e.updateChecks == nil {
		e.updateChecks = make(map[string]*updateCheck)
	}
	for key, check := range e.updateChecks {
		if check.sessionKey == msg.SessionKey && check.userID == msg.UserID {
			if check.ctx.Err() == nil {
				e.updateServiceMu.Unlock()
				cancel()
				return nil, key, nil, errUpdateCheckInProgress
			}
			check.cancel()
			delete(e.updateChecks, key)
		}
	}
	e.updateChecks[token] = &updateCheck{sessionKey: msg.SessionKey, userID: msg.UserID, cancel: cancel, ctx: ctx}
	e.updateServiceMu.Unlock()
	return ctx, token, func() { cancel(); e.updateServiceMu.Lock(); delete(e.updateChecks, token); e.updateServiceMu.Unlock() }, nil
}
func (e *Engine) cancelUpdateCheck(msg *Message, token string) bool {
	e.updateServiceMu.Lock()
	defer e.updateServiceMu.Unlock()
	check := e.updateChecks[token]
	if check != nil && check.sessionKey == msg.SessionKey && check.userID == msg.UserID {
		check.cancel()
		return true
	}
	return false
}
