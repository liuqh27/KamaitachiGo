package common

// HttpResponse HTTP响应结构
type HttpResponse struct {
	Code    int         `json:"code"`    // 响应码
	Message string      `json:"message"` // 响应消息
	Data    interface{} `json:"data"`    // 响应数据
}

// NewSuccessResponse 创建成功响应
func NewSuccessResponse(data interface{}) *HttpResponse {
	return &HttpResponse{
		Code:    200,
		Message: "success",
		Data:    data,
	}
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(code int, message string) *HttpResponse {
	return &HttpResponse{
		Code:    code,
		Message: message,
		Data:    nil,
	}
}

// NewFailureResponse 创建失败响应
func NewFailureResponse() *HttpResponse {
	return &HttpResponse{
		Code:    500,
		Message: "internal server error",
		Data:    nil,
	}
}

