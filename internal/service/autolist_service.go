package service

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type AutolistService struct {
	repo        *repository.AutolistRepository
	postRepo    *repository.PostRepository
	publishServ *PostPublishService
}

func NewAutolistService(repo *repository.AutolistRepository, postRepo *repository.PostRepository, pubServ *PostPublishService) *AutolistService {
	return &AutolistService{
		repo:        repo,
		postRepo:    postRepo,
		publishServ: pubServ,
	}
}

func (s *AutolistService) CreateAutolist(ctx context.Context, brandID bson.ObjectID, req models.CreateAutolistRequest) (*models.Autolist, error) {
	list := &models.Autolist{
		BrandID:         brandID,
		Name:            req.Name,
		Description:     req.Description,
		TargetPlatforms: req.TargetPlatforms,
		TimeSlots:       req.TimeSlots,
		Posts:           []models.AutolistPost{},
		IsCircular:      req.IsCircular,
	}
	if err := s.repo.CreateAutolist(ctx, list); err != nil {
		return nil, utils.InternalServerError("Failed to create autolist")
	}
	return list, nil
}

func (s *AutolistService) AddPostToAutolist(ctx context.Context, autolistID, brandID bson.ObjectID, req models.AddAutolistPostRequest) (*models.Autolist, error) {
	list, err := s.repo.FindAutolistByID(ctx, autolistID)
	if err != nil || list == nil || list.BrandID != brandID {
		return nil, utils.NotFound("Autolist not found")
	}

	post := models.AutolistPost{
		ID:               uuid.NewString(),
		Content:          req.Content,
		Media:            req.Media,
		Status:           "active",
		RepetitionsCount: 0,
		MaxRepetitions:   req.MaxRepetitions,
		Order:            len(list.Posts),
	}

	list.Posts = append(list.Posts, post)
	return s.repo.UpdateAutolist(ctx, autolistID, bson.M{"posts": list.Posts})
}

func (s *AutolistService) ListAutolists(ctx context.Context, brandID bson.ObjectID) ([]models.Autolist, error) {
	return s.repo.FindAutolistsByBrand(ctx, brandID)
}

func (s *AutolistService) DeleteAutolist(ctx context.Context, id, brandID bson.ObjectID) error {
	deleted, err := s.repo.DeleteAutolist(ctx, id, brandID)
	if err != nil {
		return utils.InternalServerError("Failed to delete autolist")
	}
	if !deleted {
		return utils.NotFound("Autolist not found")
	}
	return nil
}

// RSS Feeds
type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

type rssChannel struct {
	Title string    `xml:"title"`
	Items []rssItem `xml:"item"`
}

type rssFeedXML struct {
	Channel rssChannel `xml:"channel"`
}

func (s *AutolistService) CreateRSSFeed(ctx context.Context, brandID bson.ObjectID, req models.CreateRSSFeedRequest) (*models.RSSFeed, error) {
	interval := req.CheckIntervalMinutes
	if interval < 15 {
		interval = 60
	}
	template := req.Template
	if template == "" {
		template = "{title} - {link}"
	}

	feed := &models.RSSFeed{
		BrandID:              brandID,
		FeedURL:              req.FeedURL,
		TargetPlatforms:      req.TargetPlatforms,
		Template:             template,
		CheckIntervalMinutes: interval,
		AutoPublish:          req.AutoPublish,
	}

	if err := s.repo.CreateRSSFeed(ctx, feed); err != nil {
		return nil, utils.InternalServerError("Failed to register RSS feed")
	}
	return feed, nil
}

func (s *AutolistService) ListRSSFeeds(ctx context.Context, brandID bson.ObjectID) ([]models.RSSFeed, error) {
	return s.repo.FindRSSFeedsByBrand(ctx, brandID)
}

func (s *AutolistService) DeleteRSSFeed(ctx context.Context, id, brandID bson.ObjectID) error {
	deleted, err := s.repo.DeleteRSSFeed(ctx, id, brandID)
	if err != nil {
		return utils.InternalServerError("Failed to delete RSS feed")
	}
	if !deleted {
		return utils.NotFound("RSS feed not found")
	}
	return nil
}

func (s *AutolistService) SyncRSSFeed(ctx context.Context, feed *models.RSSFeed, userID bson.ObjectID) (int, error) {
	resp, err := http.Get(feed.FeedURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var parsed rssFeedXML
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return 0, err
	}

	createdCount := 0
	for _, item := range parsed.Channel.Items {
		guid := item.GUID
		if guid == "" {
			guid = item.Link
		}
		if guid == feed.LastPostGUID {
			break // Caught up
		}

		caption := feed.Template
		caption = strings.ReplaceAll(caption, "{title}", item.Title)
		caption = strings.ReplaceAll(caption, "{link}", item.Link)
		caption = strings.ReplaceAll(caption, "{description}", item.Description)

		now := time.Now()
		post := &models.Post{
			UserID:            userID,
			BrandID:           &feed.BrandID,
			Title:             item.Title,
			Caption:           caption,
			Status:            models.PostStatusDraft,
			SelectedPlatforms: feed.TargetPlatforms,
			PostType:          "text",
			Visibility:        "public",
			CreatedAt:         now,
			UpdatedAt:         now,
		}

		if feed.AutoPublish {
			post.Status = models.PostStatusScheduled
			post.ScheduledTime = &now
		}

		if err := s.postRepo.Create(ctx, post); err == nil {
			createdCount++
		}
	}

	if len(parsed.Channel.Items) > 0 {
		topGUID := parsed.Channel.Items[0].GUID
		if topGUID == "" {
			topGUID = parsed.Channel.Items[0].Link
		}
		_ = s.repo.UpdateRSSFeed(ctx, feed.ID, bson.M{
			"lastPostGuid":  topGUID,
			"lastCheckedAt": time.Now(),
		})
	}

	return createdCount, nil
}
