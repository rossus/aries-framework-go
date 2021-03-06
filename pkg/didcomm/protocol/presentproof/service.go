/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package presentproof

import (
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/hyperledger/aries-framework-go/pkg/common/log"
	"github.com/hyperledger/aries-framework-go/pkg/didcomm/common/service"
	"github.com/hyperledger/aries-framework-go/pkg/storage"
)

const (
	// Name defines the protocol name
	Name = "present-proof"
	// Spec defines the protocol spec
	Spec = "https://didcomm.org/present-proof/1.0/"
	// ProposePresentationMsgType defines the protocol propose-presentation message type.
	ProposePresentationMsgType = Spec + "propose-presentation"
	// RequestPresentationMsgType defines the protocol request-presentation message type.
	RequestPresentationMsgType = Spec + "request-presentation"
	// PresentationMsgType defines the protocol presentation message type.
	PresentationMsgType = Spec + "presentation"
	// AckMsgType defines the protocol ack message type.
	AckMsgType = Spec + "ack"
	// ProblemReportMsgType defines the protocol problem-report message type.
	ProblemReportMsgType = Spec + "problem-report"
	// PresentationPreviewMsgType defines the protocol presentation-preview inner object type.
	PresentationPreviewMsgType = Spec + "presentation-preview"
)

const stateNameKey = "state_name_"

var logger = log.New("aries-framework/presentproof/service")

// customError is a wrapper to determine custom error against internal error
type customError struct{ error }

// transitionalPayload keeps payload needed for Continue function to proceed with the action
type transitionalPayload struct {
	// protocol state machine identifier
	PIID      string
	StateName string
	Msg       service.DIDCommMsgMap
	MyDID     string
	TheirDID  string
}

// metaData type to store data for internal usage
type metaData struct {
	transitionalPayload
	state    state
	msgClone service.DIDCommMsg
	inbound  bool
	// err is used to determine whether callback was stopped
	// e.g the user received an action event and executes Stop(err) function
	// in that case `err` is equal to `err` which was passing to Stop function
	err error
}

// Provider contains dependencies for the protocol and is typically created by using aries.Context()
type Provider interface {
	Messenger() service.Messenger
	StorageProvider() storage.Provider
}

// Service for the presentproof protocol
type Service struct {
	service.Action
	service.Message
	store     storage.Store
	callbacks chan *metaData
	messenger service.Messenger
}

// New returns the presentproof service
func New(p Provider) (*Service, error) {
	store, err := p.StorageProvider().OpenStore(Name)
	if err != nil {
		return nil, err
	}

	svc := &Service{
		messenger: p.Messenger(),
		store:     store,
		callbacks: make(chan *metaData),
	}

	// start the listener
	go svc.startInternalListener()

	return svc, nil
}

// HandleInbound handles inbound message (presentproof protocol)
func (s *Service) HandleInbound(msg service.DIDCommMsg, myDID, theirDID string) (string, error) {
	aEvent := s.ActionEvent()

	// throw error if there is no action event registered for inbound messages
	if aEvent == nil {
		return "", errors.New("no clients are registered to handle the message")
	}

	md, err := s.doHandle(msg, false)
	if err != nil {
		return "", fmt.Errorf("doHandle: %w", err)
	}

	// sets inbound payload
	md.inbound = true
	md.MyDID = myDID
	md.TheirDID = theirDID

	// trigger action event based on message type for inbound messages
	if canTriggerActionEvents(msg) {
		aEvent <- s.newDIDCommActionMsg(md)

		return "", nil
	}

	// if no action event is triggered, continue the execution
	return "", s.handle(md)
}

// HandleOutbound handles outbound message (presentproof protocol)
func (s *Service) HandleOutbound(msg service.DIDCommMsg, myDID, theirDID string) error {
	md, err := s.doHandle(msg, true)
	if err != nil {
		return fmt.Errorf("doHandle: %w", err)
	}

	// sets outbound payload
	md.MyDID = myDID
	md.TheirDID = theirDID

	return s.handle(md)
}

func (s *Service) getCurrentStateNameAndPIID(msg service.DIDCommMsg) (string, string, error) {
	piID, err := getPIID(msg)
	if errors.Is(err, service.ErrThreadIDNotFound) {
		piID = uuid.New().String()

		return piID, stateNameStart, msg.SetID(piID)
	}

	if err != nil {
		return "", "", fmt.Errorf("piID: %w", err)
	}

	stateName, err := s.currentStateName(piID)
	if err != nil {
		return "", "", fmt.Errorf("currentStateName: %w", err)
	}

	return piID, stateName, nil
}

func (s *Service) doHandle(msg service.DIDCommMsg, outbound bool) (*metaData, error) {
	piID, stateName, err := s.getCurrentStateNameAndPIID(msg)
	if err != nil {
		return nil, fmt.Errorf("getCurrentStateNameAndPIID: %w", err)
	}

	current := stateFromName(stateName)

	next, err := nextState(msg, outbound)
	if err != nil {
		return nil, fmt.Errorf("nextState: %w", err)
	}

	if !current.CanTransitionTo(next) {
		return nil, fmt.Errorf("invalid state transition: %s -> %s", current.Name(), next.Name())
	}

	return &metaData{
		transitionalPayload: transitionalPayload{
			StateName: next.Name(),
			Msg:       msg.(service.DIDCommMsgMap),
			PIID:      piID,
		},
		state:    next,
		msgClone: msg.Clone(),
	}, nil
}

// startInternalListener listens to messages in go channel for callback messages from clients.
func (s *Service) startInternalListener() {
	for msg := range s.callbacks {
		// if no error do handle
		if msg.err == nil {
			msg.err = s.handle(msg)
		}

		// no error - continue
		if msg.err == nil {
			continue
		}

		msg.state = &abandoning{Code: codeInternalError}

		if err := s.handle(msg); err != nil {
			logger.Errorf("listener handle: %s", err)
		}
	}
}

func isNoOp(s state) bool {
	_, ok := s.(*noOp)
	return ok
}

func (s *Service) handle(md *metaData) error {
	var current = md.state

	for !isNoOp(current) {
		next, action, err := s.execute(current, md)
		if err != nil {
			return fmt.Errorf("execute: %w", err)
		}

		if !isNoOp(next) && !current.CanTransitionTo(next) {
			return fmt.Errorf("invalid state transition: %s --> %s", current.Name(), next.Name())
		}

		current = next

		if err := s.saveStateName(md.PIID, current.Name()); err != nil {
			return fmt.Errorf("failed to persist state %s: %w", current.Name(), err)
		}

		if err := action(s.messenger); err != nil {
			return fmt.Errorf("action %s: %w", md.state.Name(), err)
		}
	}

	return nil
}

func getPIID(msg service.DIDCommMsg) (string, error) {
	if pthID := msg.ParentThreadID(); pthID != "" {
		return pthID, nil
	}

	return msg.ThreadID()
}

func (s *Service) saveStateName(piID, stateName string) error {
	return s.store.Put(stateNameKey+piID, []byte(stateName))
}

func (s *Service) currentStateName(piID string) (string, error) {
	src, err := s.store.Get(stateNameKey + piID)
	if errors.Is(err, storage.ErrDataNotFound) {
		return stateNameStart, nil
	}

	return string(src), err
}

// nolint: gocyclo
// stateFromName returns the state by given name.
func stateFromName(name string) state {
	switch name {
	case stateNameStart:
		return &start{}
	case stateNameAbandoning:
		return &abandoning{}
	case stateNameDone:
		return &done{}
	case stateNameRequestSent:
		return &requestSent{}
	case stateNamePresentationReceived:
		return &presentationReceived{}
	case stateNameProposePresentationReceived:
		return &proposePresentationReceived{}
	case stateNameRequestReceived:
		return &requestReceived{}
	case stateNamePresentationSent:
		return &presentationSent{}
	case stateNameProposePresentationSent:
		return &proposePresentationSent{}
	default:
		return &noOp{}
	}
}

// nolint: gocyclo
func nextState(msg service.DIDCommMsg, outbound bool) (state, error) {
	switch msg.Type() {
	case RequestPresentationMsgType:
		if outbound {
			return &requestSent{}, nil
		}

		return &requestReceived{}, nil
	case ProposePresentationMsgType:
		if outbound {
			return &proposePresentationSent{}, nil
		}

		return &proposePresentationReceived{}, nil
	case PresentationMsgType:
		return &presentationReceived{}, nil
	case ProblemReportMsgType:
		return &abandoning{}, nil
	case AckMsgType:
		return &done{}, nil
	default:
		return nil, fmt.Errorf("unrecognized msgType: %s", msg.Type())
	}
}

// canTriggerActionEvents checks if the incoming message can trigger an action event
func canTriggerActionEvents(msg service.DIDCommMsg) bool {
	return msg.Type() == PresentationMsgType ||
		msg.Type() == ProposePresentationMsgType ||
		msg.Type() == RequestPresentationMsgType
}

func (s *Service) processCallback(msg *metaData) {
	// pass the callback data to internal channel. This is created to unblock consumer go routine and wrap the callback
	// channel internally.
	s.callbacks <- msg
}

// newDIDCommActionMsg creates new DIDCommAction message
func (s *Service) newDIDCommActionMsg(md *metaData) service.DIDCommAction {
	// create the message for the channel
	// trigger the registered action event
	return service.DIDCommAction{
		ProtocolName: Name,
		Message:      md.msgClone,
		Continue: func(opt interface{}) {
			s.processCallback(md)
		},
		Stop: func(cErr error) {
			md.err = customError{error: cErr}
			s.processCallback(md)
		},
	}
}

func (s *Service) execute(next state, md *metaData) (state, stateAction, error) {
	s.sendMsgEvents(&service.StateMsg{
		ProtocolName: Name,
		Type:         service.PreState,
		Msg:          md.msgClone,
		StateID:      next.Name(),
	})

	defer s.sendMsgEvents(&service.StateMsg{
		ProtocolName: Name,
		Type:         service.PostState,
		Msg:          md.msgClone,
		StateID:      next.Name(),
	})

	exec := next.ExecuteOutbound
	if md.inbound {
		exec = next.ExecuteInbound
	}

	return exec(md)
}

// sendMsgEvents triggers the message events.
func (s *Service) sendMsgEvents(msg *service.StateMsg) {
	// trigger the message events
	for _, handler := range s.MsgEvents() {
		handler <- *msg
	}
}

// Name returns service name
func (s *Service) Name() string {
	return Name
}

// Accept msg checks the msg type
func (s *Service) Accept(msgType string) bool {
	switch msgType {
	case ProposePresentationMsgType, RequestPresentationMsgType,
		PresentationMsgType, AckMsgType, ProblemReportMsgType:
		return true
	}

	return false
}
