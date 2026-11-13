package core

import "encoding/json"

// TurnRecoveryInput is the already prepared input of an ordinary chat turn.
// Recovery must reuse the native conversation and reconcile its last turn
// before deciding whether any new model execution is necessary.
type TurnRecoveryInput struct {
	Prompt     string            `json:"prompt"`
	Images     []ImageAttachment `json:"images,omitempty"`
	Files      []FileAttachment  `json:"files,omitempty"`
	Options    TurnOptions       `json:"options"`
	UseOptions bool              `json:"use_options"`
}

type TurnRecoverySupporter interface{ SupportsTurnRecovery() bool }

// TurnRecoverySession is optional. Checkpoints identify the native conversation
// and the boundary BEFORE a submission, including when the submission receipt
// was lost. ResumeInterruptedTurn emits normal Events: a completed native turn
// only replays its answer; an interrupted turn continues its existing history.
// Save must succeed before starting a continuation so another restart can
// reconcile that submission too. It must never grant pending permissions.
type TurnRecoverySession interface {
	PrepareTurnRecovery(input TurnRecoveryInput) (json.RawMessage, error)
	ResumeInterruptedTurn(checkpoint json.RawMessage, input TurnRecoveryInput, save func(json.RawMessage) error) error
}

// PersistentPreviewPlatform preserves the same visible answer across restart.
// The platform owns its opaque handle and must checkpoint ordering counters
// BEFORE issuing updates, including updates whose remote outcome is unknown.
// Snapshots contain routing and ordering metadata, never credentials.
type PersistentPreviewPlatform interface {
	SnapshotTurnReplyCtx(replyCtx any) (json.RawMessage, error)
	RestoreTurnReplyCtx(snapshot json.RawMessage) (any, error)
	CheckpointPreview(handle any, save func(json.RawMessage) error) error
	RestorePreview(snapshot json.RawMessage, save func(json.RawMessage) error) (any, error)
}
