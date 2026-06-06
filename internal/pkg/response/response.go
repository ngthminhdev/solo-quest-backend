package response

import "net/http"

type Response struct {
	Code       int         `json:"code"`
	Message    string      `json:"message"`
	Data       any         `json:"data,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

type Pagination struct {
	Page       int32 `json:"page"`
	PageSize   int32 `json:"page_size"`
	TotalCount int64 `json:"total_count"`
	TotalPages int   `json:"total_pages"`
}

func Success(data any) Response {
	return Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    data,
	}
}

func SuccessWithMessage(data any, message string) Response {
	return Response{
		Code:    http.StatusOK,
		Message: message,
		Data:    data,
	}
}

func Created(data any) Response {
	return Response{
		Code:    http.StatusCreated,
		Message: "Created",
		Data:    data,
	}
}

func SuccessWithPagination(data any, page int32, pageSize int32, totalCount int64) Response {
	totalPages := int(0)
	if pageSize > 0 {
		totalPages = int((totalCount + int64(pageSize) - 1) / int64(pageSize))
	}
	return Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    data,
		Pagination: &Pagination{
			Page:       page,
			PageSize:   pageSize,
			TotalCount: totalCount,
			TotalPages: totalPages,
		},
	}
}

func Error(code int, message string) Response {
	return Response{
		Code:    code,
		Message: message,
	}
}

func BadRequest(message string) Response {
	return Error(http.StatusBadRequest, message)
}

func Unauthorized() Response {
	return Error(http.StatusUnauthorized, "unauthorized")
}

func NotFound(message string) Response {
	return Error(http.StatusNotFound, message)
}

func Conflict(message string) Response {
	return Error(http.StatusConflict, message)
}

func InternalError(message string) Response {
	return Error(http.StatusInternalServerError, message)
}
