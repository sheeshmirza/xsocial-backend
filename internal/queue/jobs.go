package queue

import (
	"context"
	"encoding/json"
	"log"
)

type PostExecutor interface {
	PublishScheduledPost(ctx context.Context, postID string) error
	ExpirePublishedPlatformPost(ctx context.Context, payload ExpiryJobData) error
}

func CreateJobDispatcher(executor PostExecutor) HandlerFunc {
	return func(name string, payload []byte) error {
		ctx := context.Background()

		switch name {
		case PublishPostJobName:
			var data PublishJobData
			if err := json.Unmarshal(payload, &data); err != nil {
				return err
			}
			log.Printf("[JobDispatcher] Processing publish post job for post %s", data.PostID)
			return executor.PublishScheduledPost(ctx, data.PostID)

		case ExpirePostPublicationJobName:
			var data ExpiryJobData
			if err := json.Unmarshal(payload, &data); err != nil {
				return err
			}
			log.Printf("[JobDispatcher] Processing expire post publication job for post %s, platform %s", data.PostID, data.Platform)
			return executor.ExpirePublishedPlatformPost(ctx, data)

		default:
			log.Printf("[JobDispatcher] Unknown job name: %s", name)
			return nil
		}
	}
}
