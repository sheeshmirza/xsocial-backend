package instagram

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type,omitempty"`
	ExpiresIn   int    `json:"expires_in,omitempty"`
	UserID      int64  `json:"user_id,omitempty"`
}

type UserProfileResponse struct {
	ID                string `json:"id"`
	Username          string `json:"username"`
	Name              string `json:"name"`
	AccountType       string `json:"account_type"`
	ProfilePictureURL string `json:"profile_picture_url"`
}

type ContainerResponse struct {
	ID string `json:"id"`
}

type ContainerStatusResponse struct {
	ID         string `json:"id"`
	StatusCode string `json:"status_code"` // FINISHED, ERROR, EXPIRED, IN_PROGRESS
}

type PublishMediaResponse struct {
	ID string `json:"id"`
}

type APIErrorResponse struct {
	Error struct {
		Message      string `json:"message"`
		Type         string `json:"type"`
		Code         int    `json:"code"`
		ErrorSubcode int    `json:"error_subcode"`
		FBTraceID    string `json:"fbtrace_id"`
	} `json:"error"`
}
