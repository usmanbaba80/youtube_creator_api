package models

import "time"

type Creator struct {
	ID               int64      `json:"id"`
	CreatorID        int        `json:"creator_id"`
	CategoryID       int        `json:"category_id"`
	Name             string     `json:"name"`
	MonetizationModel *string   `json:"monetization_model,omitempty"`
	ChannelURL       string     `json:"channel_url"`
	Handle           *string    `json:"handle,omitempty"`
	YouTubeChannelID *string    `json:"youtube_channel_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

type MediaItem struct {
	ID               int64      `json:"id"`
	PublicID         string     `json:"public_id"`
	YouTubeVideoID   string     `json:"youtube_video_id"`
	URL              string     `json:"youtube_url"`
	Rank             int        `json:"rank"`
	Title            *string    `json:"title,omitempty"`
	Description      *string    `json:"description,omitempty"`
	PublishedAt      *string    `json:"published_at,omitempty"`
	DurationISO      *string    `json:"duration_iso,omitempty"`
	DurationSeconds  *int       `json:"duration_seconds,omitempty"`
	ViewCount        *int64     `json:"view_count,omitempty"`
	LikeCount        *int64     `json:"like_count,omitempty"`
	CommentCount     *int64     `json:"comment_count,omitempty"`
	ThumbnailURL     *string    `json:"thumbnail_url,omitempty"`
	BunnyThumbnailURL *string   `json:"bunny_thumbnail_url,omitempty"`
	BunnyURL         *string    `json:"bunny_url,omitempty"`
	BunnyPath        *string    `json:"bunny_path,omitempty"`
	UploadedAt       *time.Time `json:"uploaded_at,omitempty"`
}

type PlaylistSummary struct {
	ID                 int64     `json:"id"`
	PlaylistID         string    `json:"playlist_id"`
	YouTubePlaylistID  string    `json:"youtube_playlist_id"`
	URL                string    `json:"youtube_url"`
	Rank               int       `json:"rank"`
	Title              *string   `json:"title,omitempty"`
	Description        *string   `json:"description,omitempty"`
	PublishedAt        *string   `json:"published_at,omitempty"`
	ThumbnailURL       *string   `json:"thumbnail_url,omitempty"`
	BunnyThumbnailURL  *string   `json:"bunny_thumbnail_url,omitempty"`
	ItemCount          *int      `json:"item_count,omitempty"`
	ReadyItemCount     int64     `json:"ready_item_count"`
	CreatedAt          time.Time `json:"created_at"`
}

type PlaylistDetail struct {
	PlaylistSummary
	Items []PlaylistItem `json:"items"`
}

type PlaylistItem struct {
	Position         int     `json:"position"`
	YouTubeVideoID   string  `json:"youtube_video_id"`
	YouTubeURL       string  `json:"youtube_url"`
	ReuseSource      string  `json:"reuse_source"`
	Title            *string `json:"title,omitempty"`
	Description      *string `json:"description,omitempty"`
	PublishedAt      *string `json:"published_at,omitempty"`
	DurationISO      *string `json:"duration_iso,omitempty"`
	DurationSeconds  *int    `json:"duration_seconds,omitempty"`
	ThumbnailURL      *string `json:"thumbnail_url,omitempty"`
	BunnyThumbnailURL *string `json:"bunny_thumbnail_url,omitempty"`
	BunnyURL          *string `json:"bunny_url,omitempty"`
	BunnyPath         *string `json:"bunny_path,omitempty"`
	LinkedVideoID     *string `json:"linked_video_id,omitempty"`
	LinkedShortID     *string `json:"linked_short_id,omitempty"`
}
