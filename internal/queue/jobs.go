package queue

import (
	"context"
	"encoding/json"
	"log/slog"

	"xsocial-backend/internal/utils"
)

type PostExecutor interface {
	PublishScheduledPost(ctx context.Context, postID string) error
	ExpirePublishedPlatformPost(ctx context.Context, payload ExpiryJobData) error
}

func CreateJobDispatcher(executor PostExecutor) HandlerFunc {
	return func(name string, payload []byte) error {
		switch name {
		case PublishPostJobName:
			var data PublishJobData
			if err := json.Unmarshal(payload, &data); err != nil {
				return err
			}
			ctx := utils.ContextWithTraceparent(context.Background(), data.Traceparent, data.RequestID)
			slog.InfoContext(ctx, "job_dispatch_publish_post", "post_id", data.PostID)
			return executor.PublishScheduledPost(ctx, data.PostID)

		case ExpirePostPublicationJobName:
			var data ExpiryJobData
			if err := json.Unmarshal(payload, &data); err != nil {
				return err
			}
			ctx := utils.ContextWithTraceparent(context.Background(), data.Traceparent, data.RequestID)
			slog.InfoContext(ctx, "job_dispatch_expire_post", "post_id", data.PostID, "platform", data.Platform)
			return executor.ExpirePublishedPlatformPost(ctx, data)

		default:
			slog.Warn("job_dispatch_unknown", "name", name)
			return nil
		}
	}
}
