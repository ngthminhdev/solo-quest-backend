package dto

type RegisterDeviceTokenRequest struct {
	Token      string  `json:"token" binding:"required"`
	Platform   string  `json:"platform" binding:"required"`
	DeviceName *string `json:"device_name"`
}

type DeviceTokenResponse struct {
	ID         string  `json:"id"`
	UserID     string  `json:"user_id"`
	Token      string  `json:"token"`
	Platform   string  `json:"platform"`
	DeviceName *string `json:"device_name,omitempty"`
	IsActive   bool    `json:"is_active"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
}

type SendTestNotificationRequest struct {
	Title string            `json:"title"`
	Body  string            `json:"body"`
	Data  map[string]string `json:"data"`
}
