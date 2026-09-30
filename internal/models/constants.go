package models

import "strings"

// Platform Constants
const (
	PlatformInstagram = "INSTAGRAM"
	PlatformLinkedIn  = "LINKEDIN"
)

// Post Status Constants
const (
	PostStatusDraft     = "draft"
	PostStatusScheduled = "scheduled"
	PostStatusPublished = "published"
	PostStatusFailed    = "failed"
)

// Recurrence Constants
const (
	RecurrenceOnce    = "once"
	RecurrenceHourly  = "hourly"
	RecurrenceDaily   = "daily"
	RecurrenceWeekly  = "weekly"
	RecurrenceMonthly = "monthly"
	RecurrenceYearly  = "yearly"
)

// Recurrence Unit Constants
const (
	RecurrenceUnitMinute = "minute"
	RecurrenceUnitHour   = "hour"
	RecurrenceUnitDay    = "day"
	RecurrenceUnitWeek   = "week"
	RecurrenceUnitMonth  = "month"
)

// Account Type Constants
const (
	InstagramAccountTypeBusiness = "business"
	InstagramAccountTypeCreator  = "creator"

	LinkedInAccountTypeOrganization = "organization"
	LinkedInAccountTypePerson       = "person"
)

func IsLinkedInOrganization(accountType string) bool {
	return strings.ToLower(strings.TrimSpace(accountType)) == LinkedInAccountTypeOrganization
}

func NormalizeAccountTypeByPlatform(platform, accountType string) string {
	normPlatform := strings.ToLower(strings.TrimSpace(platform))
	normType := strings.ToLower(strings.TrimSpace(accountType))

	if normPlatform == "instagram" {
		if normType == InstagramAccountTypeBusiness {
			return InstagramAccountTypeBusiness
		}
		return InstagramAccountTypeCreator
	}

	if normPlatform == "linkedin" {
		if normType == LinkedInAccountTypeOrganization {
			return LinkedInAccountTypeOrganization
		}
		return LinkedInAccountTypePerson
	}

	return normType
}
