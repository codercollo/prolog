package log_v1

import (
	"fmt"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/status"
)

// ErrOffsetOutOfRange indicates a requested offset is outside the log's bounds.
type ErrOffsetOutOfRange struct {
	Offset uint64
}

// GRPCStatus returns a 404 grpc status with localize error details.
func (e ErrOffsetOutOfRange) GRPCStatus() *status.Status {
	st := status.New(404,
		fmt.Sprintf("offset out of range: %d", e.Offset),
	)

	msg := fmt.Sprintf(
		"The requested offset is outside the log's range: %d",
		e.Offset,
	)

	d := &errdetails.LocalizedMessage{
		Locale:  "en-US",
		Message: msg,
	}

	// Attach details or fall back to base status
	std, err := st.WithDetails(d)
	if err != nil {
		return st
	}

	return std
}

// Error returns the gRPC status error string
func (e ErrOffsetOutOfRange) Error() string {
	return e.GRPCStatus().Err().Error()
}
