package models_test

import (
	"strings"
	"testing"

	"xsocial-backend/internal/models"
)

func TestCreatePostInput_Validate(t *testing.T) {
	t.Run("Valid Post Input", func(t *testing.T) {
		in := models.CreatePostInput{
			Title:      "My First Post",
			Caption:    "Hello world!",
			PostType:   "mixed",
			Visibility: "public",
			Timezone:   "UTC",
			Recurrence: models.RecurrenceOnce,
		}
		if err := in.Validate(); err != nil {
			t.Errorf("expected valid input, got: %v", err)
		}
	})

	t.Run("Title Too Long", func(t *testing.T) {
		in := models.CreatePostInput{
			Title: strings.Repeat("A", 501),
		}
		if err := in.Validate(); err == nil {
			t.Error("expected error for title exceeding 500 chars")
		}
	})

	t.Run("Invalid Post Type", func(t *testing.T) {
		in := models.CreatePostInput{
			PostType: "malicious_type",
		}
		if err := in.Validate(); err == nil {
			t.Error("expected error for invalid post type")
		}
	})

	t.Run("Invalid Timezone", func(t *testing.T) {
		in := models.CreatePostInput{
			Timezone: "Fantasy/Narnia",
		}
		if err := in.Validate(); err == nil {
			t.Error("expected error for invalid timezone")
		}
	})

	t.Run("Invalid Media URL Scheme", func(t *testing.T) {
		in := models.CreatePostInput{
			Media: []models.MediaItem{
				{Type: "image", URL: "ftp://unsafe-server/img.jpg"},
			},
		}
		if err := in.Validate(); err == nil {
			t.Error("expected error for ftp:// scheme")
		}
	})
}

func TestCreateBrandRequest_Validate(t *testing.T) {
	t.Run("Valid Brand Request", func(t *testing.T) {
		req := models.CreateBrandRequest{
			Name:     "Acme Corp",
			Currency: "USD",
			Timezone: "America/New_York",
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid, got: %v", err)
		}
	})

	t.Run("Empty Brand Name", func(t *testing.T) {
		req := models.CreateBrandRequest{
			Name: "   ",
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty brand name")
		}
	})

	t.Run("Invalid Currency Code", func(t *testing.T) {
		req := models.CreateBrandRequest{
			Name:     "Brand",
			Currency: "INVALID",
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for invalid currency")
		}
	})
}

func TestCreateApiKeyRequest_Validate(t *testing.T) {
	t.Run("Valid Api Key Request", func(t *testing.T) {
		req := models.CreateApiKeyRequest{
			Name:          "Production Worker",
			ExpiresInDays: 30,
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid, got: %v", err)
		}
	})

	t.Run("Empty Key Name", func(t *testing.T) {
		req := models.CreateApiKeyRequest{
			Name: "",
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty key name")
		}
	})

	t.Run("Negative Expiration", func(t *testing.T) {
		req := models.CreateApiKeyRequest{
			Name:          "Key",
			ExpiresInDays: -10,
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for negative expiresInDays")
		}
	})
}

func TestUpdatePostInput_Validate(t *testing.T) {
	t.Run("Valid Update Input", func(t *testing.T) {
		title := "Updated title"
		postType := "reel"
		visibility := "public"
		in := models.UpdatePostInput{
			Title:      &title,
			PostType:   &postType,
			Visibility: &visibility,
		}
		if err := in.Validate(); err != nil {
			t.Errorf("expected valid update input, got: %v", err)
		}
	})

	t.Run("Invalid Post Type", func(t *testing.T) {
		badType := "unsupported_type"
		in := models.UpdatePostInput{
			PostType: &badType,
		}
		if err := in.Validate(); err == nil {
			t.Error("expected error for unsupported post type")
		}
	})

	t.Run("Title Too Long", func(t *testing.T) {
		longTitle := strings.Repeat("A", 501)
		in := models.UpdatePostInput{
			Title: &longTitle,
		}
		if err := in.Validate(); err == nil {
			t.Error("expected error for title exceeding 500 characters")
		}
	})
}

func TestBulkIDsInput_Validate(t *testing.T) {
	t.Run("Valid IDs", func(t *testing.T) {
		in := models.BulkIDsInput{
			IDs: []string{"id1", "id2"},
		}
		if err := in.Validate(); err != nil {
			t.Errorf("expected valid bulk input, got: %v", err)
		}
	})

	t.Run("Empty IDs", func(t *testing.T) {
		in := models.BulkIDsInput{
			IDs: []string{},
		}
		if err := in.Validate(); err == nil {
			t.Error("expected error for empty IDs")
		}
	})

	t.Run("Exceeds Limit", func(t *testing.T) {
		ids := make([]string, 101)
		for i := range ids {
			ids[i] = "id"
		}
		in := models.BulkIDsInput{
			IDs: ids,
		}
		if err := in.Validate(); err == nil {
			t.Error("expected error for exceeding 100 IDs")
		}
	})
}

func TestResharePostRequest_Validate(t *testing.T) {
	t.Run("Valid Reshare Request", func(t *testing.T) {
		caption := "Great post!"
		req := models.ResharePostRequest{
			TargetPlatforms:    []string{"linkedin"},
			NewCaption:         &caption,
			PublishImmediately: true,
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid reshare request, got: %v", err)
		}
	})

	t.Run("Missing Target Accounts and Platforms", func(t *testing.T) {
		req := models.ResharePostRequest{
			PublishImmediately: true,
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for missing targets")
		}
	})
}

func TestUpdateBrandRequest_Validate(t *testing.T) {
	t.Run("Valid Update", func(t *testing.T) {
		name := "New Brand Name"
		curr := "EUR"
		req := models.UpdateBrandRequest{
			Name:     &name,
			Currency: &curr,
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid update, got: %v", err)
		}
	})

	t.Run("Empty Name", func(t *testing.T) {
		name := "   "
		req := models.UpdateBrandRequest{
			Name: &name,
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty name")
		}
	})

	t.Run("Invalid Currency", func(t *testing.T) {
		curr := "US"
		req := models.UpdateBrandRequest{
			Currency: &curr,
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for invalid currency length")
		}
	})
}

func TestAddBrandMemberRequest_Validate(t *testing.T) {
	t.Run("Valid Member Request", func(t *testing.T) {
		req := models.AddBrandMemberRequest{
			Email: "member@example.com",
			Name:  "Jane Doe",
			Role:  models.BrandRoleEditor,
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid, got: %v", err)
		}
	})

	t.Run("Invalid Email", func(t *testing.T) {
		req := models.AddBrandMemberRequest{
			Email: "invalid-email",
			Role:  models.BrandRoleViewer,
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for invalid email")
		}
	})

	t.Run("Invalid Role", func(t *testing.T) {
		req := models.AddBrandMemberRequest{
			Email: "test@example.com",
			Role:  "superadmin",
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for invalid role")
		}
	})
}

func TestSignupRequest_Validate(t *testing.T) {
	t.Run("Valid Signup", func(t *testing.T) {
		req := models.SignupRequest{
			Email:    "User@Example.com",
			Password: "ValidPassword123!",
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid, got: %v", err)
		}
		if req.Email != "user@example.com" {
			t.Errorf("expected email to be lowercased, got: %s", req.Email)
		}
	})

	t.Run("Empty Email", func(t *testing.T) {
		req := models.SignupRequest{
			Email:    "  ",
			Password: "ValidPassword123!",
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty email")
		}
	})

	t.Run("Invalid Email Format", func(t *testing.T) {
		req := models.SignupRequest{
			Email:    "notanemail",
			Password: "ValidPassword123!",
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for invalid email")
		}
	})

	t.Run("Password Too Short", func(t *testing.T) {
		req := models.SignupRequest{
			Email:    "test@example.com",
			Password: "short",
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for short password")
		}
	})

	t.Run("Password Too Long", func(t *testing.T) {
		req := models.SignupRequest{
			Email:    "test@example.com",
			Password: strings.Repeat("A", 129),
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for long password")
		}
	})
}

func TestLoginRequest_Validate(t *testing.T) {
	t.Run("Valid Login", func(t *testing.T) {
		req := models.LoginRequest{
			Email:    "user@example.com",
			Password: "password123",
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid, got: %v", err)
		}
	})

	t.Run("Empty Password", func(t *testing.T) {
		req := models.LoginRequest{
			Email:    "user@example.com",
			Password: "",
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty password")
		}
	})
}

func TestRefreshTokenRequest_Validate(t *testing.T) {
	t.Run("Valid Refresh Token", func(t *testing.T) {
		req := models.RefreshTokenRequest{RefreshToken: "valid-token"}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid, got: %v", err)
		}
	})

	t.Run("Empty Token", func(t *testing.T) {
		req := models.RefreshTokenRequest{RefreshToken: "  "}
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty refresh token")
		}
	})

	t.Run("Token Too Long", func(t *testing.T) {
		req := models.RefreshTokenRequest{RefreshToken: strings.Repeat("x", 1025)}
		if err := req.Validate(); err == nil {
			t.Error("expected error for token exceeding 1024 chars")
		}
	})
}

func TestVerifyEmailRequest_Validate(t *testing.T) {
	t.Run("Valid Token", func(t *testing.T) {
		req := models.VerifyEmailRequest{Token: "abc123token"}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid, got: %v", err)
		}
	})

	t.Run("Empty Token", func(t *testing.T) {
		req := models.VerifyEmailRequest{Token: " "}
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty token")
		}
	})
}

func TestForgotPasswordRequest_Validate(t *testing.T) {
	t.Run("Valid Request", func(t *testing.T) {
		req := models.ForgotPasswordRequest{Email: "user@example.com"}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid, got: %v", err)
		}
	})

	t.Run("Invalid Email", func(t *testing.T) {
		req := models.ForgotPasswordRequest{Email: "invalid-email"}
		if err := req.Validate(); err == nil {
			t.Error("expected error for invalid email")
		}
	})
}

func TestResetPasswordRequest_Validate(t *testing.T) {
	t.Run("Valid Request", func(t *testing.T) {
		req := models.ResetPasswordRequest{
			Token:       "valid-token",
			NewPassword: "newSecurePassword123",
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid, got: %v", err)
		}
	})

	t.Run("Password Too Short", func(t *testing.T) {
		req := models.ResetPasswordRequest{
			Token:       "valid-token",
			NewPassword: "short",
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for short password")
		}
	})
}

func TestUpdateProfileRequest_Validate(t *testing.T) {
	t.Run("Valid Profile", func(t *testing.T) {
		req := models.UpdateProfileRequest{
			FullName: "Alice Smith",
			Timezone: "America/New_York",
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid, got: %v", err)
		}
	})

	t.Run("Null Byte Stripping", func(t *testing.T) {
		req := models.UpdateProfileRequest{
			FullName: "Alice\x00 Smith",
			Timezone: "UTC",
		}
		if err := req.Validate(); err != nil {
			t.Fatalf("expected valid, got: %v", err)
		}
		if req.FullName != "Alice Smith" {
			t.Errorf("expected null bytes stripped, got: %q", req.FullName)
		}
	})

	t.Run("Invalid Timezone", func(t *testing.T) {
		req := models.UpdateProfileRequest{
			FullName: "Alice Smith",
			Timezone: "Invalid/Zone",
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for invalid timezone")
		}
	})
}

func TestChangePasswordRequest_Validate(t *testing.T) {
	t.Run("Valid Change", func(t *testing.T) {
		req := models.ChangePasswordRequest{
			OldPassword: "oldPassword123",
			NewPassword: "newPassword123",
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid, got: %v", err)
		}
	})

	t.Run("Empty Old Password", func(t *testing.T) {
		req := models.ChangePasswordRequest{
			OldPassword: "",
			NewPassword: "newPassword123",
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for missing old password")
		}
	})
}

func TestPublishPostInput_Validate(t *testing.T) {
	t.Run("Valid Input", func(t *testing.T) {
		rec := models.RecurrenceDaily
		in := models.PublishPostInput{
			Platforms:  []string{"Instagram", "LinkedIn"},
			Recurrence: &rec,
		}
		if err := in.Validate(); err != nil {
			t.Errorf("expected valid input, got: %v", err)
		}
		if in.Platforms[0] != "instagram" || in.Platforms[1] != "linkedin" {
			t.Errorf("expected lowercased platforms, got: %v", in.Platforms)
		}
	})

	t.Run("Unsupported Platform", func(t *testing.T) {
		in := models.PublishPostInput{
			Platforms: []string{"tiktok"},
		}
		if err := in.Validate(); err == nil {
			t.Error("expected error for unsupported platform")
		}
	})
}

func TestSchedulePostInput_Validate(t *testing.T) {
	t.Run("Valid Schedule", func(t *testing.T) {
		in := models.SchedulePostInput{
			Timezone:   "UTC",
			Recurrence: models.RecurrenceWeekly,
		}
		if err := in.Validate(); err != nil {
			t.Errorf("expected valid schedule, got: %v", err)
		}
	})

	t.Run("Invalid Recurrence", func(t *testing.T) {
		in := models.SchedulePostInput{
			Recurrence: "every_second",
		}
		if err := in.Validate(); err == nil {
			t.Error("expected error for invalid recurrence")
		}
	})
}

func TestApproveAndRejectPostRequest_Validate(t *testing.T) {
	t.Run("Valid Approve", func(t *testing.T) {
		req := models.ApprovePostRequest{Comment: "Looks great!"}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid, got: %v", err)
		}
	})

	t.Run("Approve Comment Too Long", func(t *testing.T) {
		req := models.ApprovePostRequest{Comment: strings.Repeat("A", 2001)}
		if err := req.Validate(); err == nil {
			t.Error("expected error for comment exceeding 2000 chars")
		}
	})

	t.Run("Valid Reject", func(t *testing.T) {
		req := models.RejectPostRequest{Reason: "Needs more details"}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid, got: %v", err)
		}
	})

	t.Run("Reject Reason Too Long", func(t *testing.T) {
		req := models.RejectPostRequest{Reason: strings.Repeat("B", 2001)}
		if err := req.Validate(); err == nil {
			t.Error("expected error for reason exceeding 2000 chars")
		}
	})
}

func TestPublicReviewActionRequest_Validate(t *testing.T) {
	t.Run("Valid Action", func(t *testing.T) {
		req := models.PublicReviewActionRequest{
			Action:       "Approve",
			ReviewerName: "John Reviewer",
			Feedback:     "LGTM",
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid, got: %v", err)
		}
		if req.Action != "approve" {
			t.Errorf("expected action to be lowercased, got: %s", req.Action)
		}
	})

	t.Run("Invalid Action", func(t *testing.T) {
		req := models.PublicReviewActionRequest{
			Action: "maybe",
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for invalid action")
		}
	})

	t.Run("Reviewer Name Too Long", func(t *testing.T) {
		req := models.PublicReviewActionRequest{
			Action:       "approve",
			ReviewerName: strings.Repeat("X", 101),
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for reviewer name exceeding 100 chars")
		}
	})
}

