package linkedin

type TokenResponse struct {
	AccessToken           string `json:"access_token"`
	ExpiresIn             int    `json:"expires_in"`
	RefreshToken          string `json:"refresh_token,omitempty"`
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in,omitempty"`
	Scope                 string `json:"scope,omitempty"`
}

type UserProfileResponse struct {
	Sub                string `json:"sub"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	EmailVerified      bool   `json:"email_verified"`
	Picture            string `json:"picture"`
	Locale             map[string]any `json:"locale,omitempty"`
	ID                 string `json:"id,omitempty"`
	LocalizedFirstName string `json:"localizedFirstName,omitempty"`
	LocalizedLastName  string `json:"localizedLastName,omitempty"`
}

type ManagedPage struct {
	ID               string         `json:"id"`
	PlatformID       string         `json:"platformId"`
	Username         string         `json:"username"`
	Name             string         `json:"name"`
	DisplayName      string         `json:"displayName"`
	ProfilePicture   string         `json:"profilePicture"`
	AccountType      string         `json:"accountType"` // "organization"
	PlatformMetadata map[string]any `json:"platformMetadata"`
}

type OrganizationDetail struct {
	ID            any            `json:"id"`
	LocalizedName any            `json:"localizedName"`
	VanityName    string         `json:"vanityName"`
	LogoV2        map[string]any `json:"logoV2"`
}

type OrganizationACLElement struct {
	Role                 string         `json:"role"`
	State                any            `json:"state"`
	Organization         any            `json:"organization"`
	OrganizationTarget   any            `json:"organizationTarget"`
	OrganizationalTarget any            `json:"organizationalTarget"`
	OrgExpanded          map[string]any `json:"organization~,omitempty"`
	TargetExpanded       map[string]any `json:"organizationalTarget~,omitempty"`
}

type OrganizationACLResponse struct {
	Elements []OrganizationACLElement `json:"elements"`
	Results  []OrganizationACLElement `json:"results"`
	Data     []OrganizationACLElement `json:"data"`
	Paging   struct {
		Count int `json:"count"`
		Start int `json:"start"`
		Total int `json:"total"`
	} `json:"paging"`
}

type HistoricalPostResponse struct {
	Elements []HistoricalPost `json:"elements"`
	Paging   struct {
		Count int `json:"count"`
		Start int `json:"start"`
		Total int `json:"total"`
	} `json:"paging"`
}

type HistoricalPost struct {
	ID             string         `json:"id"`
	CreatedAt      int64          `json:"createdAt"`
	PublishedAt    int64          `json:"publishedAt"`
	LastModifiedAt int64          `json:"lastModifiedAt"`
	Commentary     any            `json:"commentary"`
	SpecificContent map[string]any `json:"specificContent"`
}
