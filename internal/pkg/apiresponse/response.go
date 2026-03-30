package apiresponse

import "encoding/json"

type SuccessEnvelope struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
}

type ErrorEnvelope struct {
	Message   string      `json:"message"`
	ErrorCode string      `json:"error_code"`
	Details   interface{} `json:"details,omitempty"`
}

func Success(data interface{}, meta interface{}) SuccessEnvelope {
	return SuccessEnvelope{
		Message: "success",
		Data:    data,
		Meta:    meta,
	}
}

func Error(message, errorCode string, details interface{}) ErrorEnvelope {
	return ErrorEnvelope{
		Message:   message,
		ErrorCode: errorCode,
		Details:   details,
	}
}

func MustMarshalSuccess(data interface{}, meta interface{}) []byte {
	payload, err := json.Marshal(Success(data, meta))
	if err != nil {
		panic(err)
	}
	return payload
}
