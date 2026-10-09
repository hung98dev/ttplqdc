package party

import (
	"thinhthan/internal/core/id"
	v1 "thinhthan/internal/protocol/v1"
)

// Result is the single S2C_PARTY_RESULT verdict every request produces
// (ADR-0064): operation_id, status, error_code, party_id.
type Result struct {
	RequestMessageID uint32
	OperationID      id.UUID
	Status           v1.ResultStatus
	ErrorCode        v1.ErrorCode
	PartyID          id.UUID
}

func (r Result) ok(partyID id.UUID) Result {
	r.Status = v1.ResultStatus_RESULT_STATUS_SUCCESS
	r.ErrorCode = v1.ErrorCode_ERROR_CODE_UNSPECIFIED
	r.PartyID = partyID
	return r
}

func (r Result) fail(code v1.ErrorCode) Result {
	r.Status = v1.ResultStatus_RESULT_STATUS_ERROR
	r.ErrorCode = code
	return r
}

// Proto renders the 653 frame payload.
func (r Result) Proto() *v1.S2CPartyResult {
	out := &v1.S2CPartyResult{
		Result: &v1.OperationResult{
			OperationId: r.OperationID[:],
			Status:      r.Status,
			ErrorCode:   r.ErrorCode,
		},
		RequestMessageId: r.RequestMessageID,
	}
	if !r.PartyID.IsNil() {
		out.PartyId = r.PartyID[:]
	}
	return out
}
