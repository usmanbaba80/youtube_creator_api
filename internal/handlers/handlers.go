package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"youtube-creator-api/internal/config"
	"youtube-creator-api/internal/models"
	"youtube-creator-api/internal/response"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type API struct {
	DB  *pgxpool.Pool
	Cfg *config.Config
}

func (a *API) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), a.Cfg.ReadTimeout)
	defer cancel()
	if err := a.DB.Ping(ctx); err != nil {
		response.Fail(w, http.StatusServiceUnavailable, "db_unavailable", "database unavailable")
		return
	}
	response.OK(w, map[string]string{"status": "ok", "version": "v1"}, nil)
}

func (a *API) GetCreator(w http.ResponseWriter, r *http.Request) {
	creator, err := a.resolveCreator(r)
	if err != nil {
		writeResolveError(w, err)
		return
	}
	response.OK(w, creator, nil)
}

func (a *API) ListVideos(w http.ResponseWriter, r *http.Request) {
	a.listMedia(w, r, "videos", "video_id", "popular_rank")
}

func (a *API) ListShorts(w http.ResponseWriter, r *http.Request) {
	a.listMedia(w, r, "shorts", "short_id", "shorts_rank")
}

func (a *API) listMedia(w http.ResponseWriter, r *http.Request, table, idCol, rankCol string) {
	creator, err := a.resolveCreator(r)
	if err != nil {
		writeResolveError(w, err)
		return
	}
	page, limit, wantAll := a.pagination(r)

	where := "creator_row_id = $1"
	args := []interface{}{creator.ID}
	if a.Cfg.ReadyOnly {
		where += " AND transfer_status = 'done' AND bunny_url IS NOT NULL AND bunny_url <> ''"
	}

	ctx := r.Context()
	var total int64
	countSQL := "SELECT COUNT(*) FROM " + table + " WHERE " + where
	if err := a.DB.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		response.Fail(w, http.StatusInternalServerError, "query_failed", "failed to count records")
		return
	}

	page, limit, offset, ok := a.resolvePageWindow(w, page, limit, wantAll, total)
	if !ok {
		return
	}

	listSQL := `
SELECT id, ` + idCol + `, youtube_video_id, url, ` + rankCol + `,
       title, description, published_at, duration_iso, duration_seconds,
       view_count, like_count, comment_count, thumbnail_url,
       bunny_url, bunny_path, uploaded_at
FROM ` + table + `
WHERE ` + where + `
ORDER BY ` + rankCol + ` ASC, id ASC
LIMIT $2 OFFSET $3`

	rows, err := a.DB.Query(ctx, listSQL, creator.ID, limit, offset)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "query_failed", "failed to load records")
		return
	}
	defer rows.Close()

	items := make([]models.MediaItem, 0)
	for rows.Next() {
		var m models.MediaItem
		if err := rows.Scan(
			&m.ID, &m.PublicID, &m.YouTubeVideoID, &m.URL, &m.Rank,
			&m.Title, &m.Description, &m.PublishedAt, &m.DurationISO, &m.DurationSeconds,
			&m.ViewCount, &m.LikeCount, &m.CommentCount, &m.ThumbnailURL,
			&m.BunnyURL, &m.BunnyPath, &m.UploadedAt,
		); err != nil {
			response.Fail(w, http.StatusInternalServerError, "scan_failed", "failed to read records")
			return
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		response.Fail(w, http.StatusInternalServerError, "query_failed", "failed to load records")
		return
	}

	response.OK(w, items, pageMeta(page, limit, total, wantAll))
}

func (a *API) ListPlaylists(w http.ResponseWriter, r *http.Request) {
	creator, err := a.resolveCreator(r)
	if err != nil {
		writeResolveError(w, err)
		return
	}
	page, limit, wantAll := a.pagination(r)

	where := "p.creator_row_id = $1"
	if a.Cfg.ReadyOnly {
		where += " AND p.metadata_status = 'done'"
	}

	ctx := r.Context()
	var total int64
	countSQL := `SELECT COUNT(*) FROM playlists p WHERE ` + where
	if err := a.DB.QueryRow(ctx, countSQL, creator.ID).Scan(&total); err != nil {
		response.Fail(w, http.StatusInternalServerError, "query_failed", "failed to count playlists")
		return
	}

	page, limit, offset, ok := a.resolvePageWindow(w, page, limit, wantAll, total)
	if !ok {
		return
	}

	listSQL := `
SELECT p.id, p.playlist_id, p.youtube_playlist_id, p.url, p.playlist_rank,
       p.title, p.description, p.published_at, p.thumbnail_url, p.item_count, p.created_at,
       COALESCE((
         SELECT COUNT(*) FROM playlist_items pi
         WHERE pi.playlist_row_id = p.id
           AND (
             (pi.reuse_source = 'video' AND EXISTS (
                SELECT 1 FROM videos v
                WHERE v.id = pi.video_row_id AND v.transfer_status = 'done' AND v.bunny_url IS NOT NULL AND v.bunny_url <> ''
             ))
             OR (pi.reuse_source = 'short' AND EXISTS (
                SELECT 1 FROM shorts s
                WHERE s.id = pi.short_row_id AND s.transfer_status = 'done' AND s.bunny_url IS NOT NULL AND s.bunny_url <> ''
             ))
             OR (pi.reuse_source = 'none' AND pi.transfer_status = 'done' AND pi.bunny_url IS NOT NULL AND pi.bunny_url <> '')
           )
       ), 0) AS ready_item_count
FROM playlists p
WHERE ` + where + `
ORDER BY p.playlist_rank ASC, p.id ASC
LIMIT $2 OFFSET $3`

	rows, err := a.DB.Query(ctx, listSQL, creator.ID, limit, offset)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "query_failed", "failed to load playlists")
		return
	}
	defer rows.Close()

	items := make([]models.PlaylistSummary, 0)
	for rows.Next() {
		var p models.PlaylistSummary
		if err := rows.Scan(
			&p.ID, &p.PlaylistID, &p.YouTubePlaylistID, &p.URL, &p.Rank,
			&p.Title, &p.Description, &p.PublishedAt, &p.ThumbnailURL, &p.ItemCount, &p.CreatedAt,
			&p.ReadyItemCount,
		); err != nil {
			response.Fail(w, http.StatusInternalServerError, "scan_failed", "failed to read playlists")
			return
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		response.Fail(w, http.StatusInternalServerError, "query_failed", "failed to load playlists")
		return
	}

	response.OK(w, items, pageMeta(page, limit, total, wantAll))
}

func (a *API) GetPlaylist(w http.ResponseWriter, r *http.Request) {
	creator, err := a.resolveCreator(r)
	if err != nil {
		writeResolveError(w, err)
		return
	}
	playlistKey := strings.TrimSpace(chi.URLParam(r, "playlistId"))
	if playlistKey == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "playlistId is required")
		return
	}

	ctx := r.Context()
	where := "p.creator_row_id = $1 AND (p.playlist_id = $2 OR p.youtube_playlist_id = $2)"
	args := []interface{}{creator.ID, playlistKey}
	if a.Cfg.ReadyOnly {
		where += " AND p.metadata_status = 'done'"
	}

	var p models.PlaylistDetail
	err = a.DB.QueryRow(ctx, `
SELECT p.id, p.playlist_id, p.youtube_playlist_id, p.url, p.playlist_rank,
       p.title, p.description, p.published_at, p.thumbnail_url, p.item_count, p.created_at
FROM playlists p
WHERE `+where+`
LIMIT 1`, args...).Scan(
		&p.ID, &p.PlaylistID, &p.YouTubePlaylistID, &p.URL, &p.Rank,
		&p.Title, &p.Description, &p.PublishedAt, &p.ThumbnailURL, &p.ItemCount, &p.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		response.Fail(w, http.StatusNotFound, "not_found", "playlist not found")
		return
	}
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "query_failed", "failed to load playlist")
		return
	}

	itemSQL := `
SELECT pi.position, pi.youtube_video_id, pi.url, pi.reuse_source,
       COALESCE(v.title, s.title, pi.title) AS title,
       COALESCE(v.description, s.description, pi.description) AS description,
       COALESCE(v.published_at, s.published_at, pi.published_at) AS published_at,
       COALESCE(v.duration_iso, s.duration_iso, pi.duration_iso) AS duration_iso,
       COALESCE(v.duration_seconds, s.duration_seconds, pi.duration_seconds) AS duration_seconds,
       COALESCE(v.thumbnail_url, s.thumbnail_url, pi.thumbnail_url) AS thumbnail_url,
       COALESCE(v.bunny_url, s.bunny_url, pi.bunny_url) AS bunny_url,
       COALESCE(v.bunny_path, s.bunny_path, pi.bunny_path) AS bunny_path,
       v.video_id, s.short_id,
       COALESCE(v.transfer_status, s.transfer_status, pi.transfer_status) AS effective_transfer_status
FROM playlist_items pi
LEFT JOIN videos v ON v.id = pi.video_row_id
LEFT JOIN shorts s ON s.id = pi.short_row_id
WHERE pi.playlist_row_id = $1
ORDER BY pi.position ASC, pi.id ASC`

	rows, err := a.DB.Query(ctx, itemSQL, p.ID)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "query_failed", "failed to load playlist items")
		return
	}
	defer rows.Close()

	items := make([]models.PlaylistItem, 0)
	var readyCount int64
	for rows.Next() {
		var it models.PlaylistItem
		var transferStatus *string
		if err := rows.Scan(
			&it.Position, &it.YouTubeVideoID, &it.YouTubeURL, &it.ReuseSource,
			&it.Title, &it.Description, &it.PublishedAt, &it.DurationISO, &it.DurationSeconds,
			&it.ThumbnailURL, &it.BunnyURL, &it.BunnyPath, &it.LinkedVideoID, &it.LinkedShortID,
			&transferStatus,
		); err != nil {
			response.Fail(w, http.StatusInternalServerError, "scan_failed", "failed to read playlist items")
			return
		}
		ready := transferStatus != nil && *transferStatus == "done" && it.BunnyURL != nil && *it.BunnyURL != ""
		if a.Cfg.ReadyOnly && !ready {
			continue
		}
		if ready {
			readyCount++
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		response.Fail(w, http.StatusInternalServerError, "query_failed", "failed to load playlist items")
		return
	}

	p.Items = items
	p.ReadyItemCount = readyCount
	response.OK(w, p, nil)
}

type resolveError struct {
	status  int
	code    string
	message string
}

func (e *resolveError) Error() string { return e.message }

func (a *API) resolveCreator(r *http.Request) (*models.Creator, error) {
	raw := strings.TrimSpace(chi.URLParam(r, "creatorId"))
	if raw == "" {
		return nil, &resolveError{http.StatusBadRequest, "bad_request", "creatorId is required"}
	}
	creatorID, err := strconv.Atoi(raw)
	if err != nil {
		return nil, &resolveError{http.StatusBadRequest, "bad_request", "creatorId must be an integer"}
	}

	ctx := r.Context()
	categoryRaw := strings.TrimSpace(r.URL.Query().Get("category_id"))

	var row models.Creator
	if categoryRaw != "" {
		categoryID, err := strconv.Atoi(categoryRaw)
		if err != nil {
			return nil, &resolveError{http.StatusBadRequest, "bad_request", "category_id must be an integer"}
		}
		err = a.DB.QueryRow(ctx, `
SELECT id, creator_id, category_id, name, monetization_model, channel_url, handle, youtube_channel_id, created_at
FROM creators
WHERE creator_id = $1 AND category_id = $2
LIMIT 1`, creatorID, categoryID).Scan(
			&row.ID, &row.CreatorID, &row.CategoryID, &row.Name, &row.MonetizationModel,
			&row.ChannelURL, &row.Handle, &row.YouTubeChannelID, &row.CreatedAt,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &resolveError{http.StatusNotFound, "not_found", "creator not found"}
		}
		if err != nil {
			return nil, &resolveError{http.StatusInternalServerError, "query_failed", "failed to load creator"}
		}
		return &row, nil
	}

	var count int
	if err := a.DB.QueryRow(ctx, `SELECT COUNT(*) FROM creators WHERE creator_id = $1`, creatorID).Scan(&count); err != nil {
		return nil, &resolveError{http.StatusInternalServerError, "query_failed", "failed to load creator"}
	}
	if count == 0 {
		return nil, &resolveError{http.StatusNotFound, "not_found", "creator not found"}
	}
	if count > 1 {
		return nil, &resolveError{http.StatusConflict, "ambiguous_creator", "multiple creators share this creator_id; pass category_id"}
	}

	err = a.DB.QueryRow(ctx, `
SELECT id, creator_id, category_id, name, monetization_model, channel_url, handle, youtube_channel_id, created_at
FROM creators
WHERE creator_id = $1
LIMIT 1`, creatorID).Scan(
		&row.ID, &row.CreatorID, &row.CategoryID, &row.Name, &row.MonetizationModel,
		&row.ChannelURL, &row.Handle, &row.YouTubeChannelID, &row.CreatedAt,
	)
	if err != nil {
		return nil, &resolveError{http.StatusInternalServerError, "query_failed", "failed to load creator"}
	}
	return &row, nil
}

func writeResolveError(w http.ResponseWriter, err error) {
	var re *resolveError
	if errors.As(err, &re) {
		response.Fail(w, re.status, re.code, re.message)
		return
	}
	response.Fail(w, http.StatusInternalServerError, "internal_error", "unexpected error")
}

func (a *API) pagination(r *http.Request) (page, limit int, wantAll bool) {
	page = 1
	limit = a.Cfg.DefaultPageSize

	allRaw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("all")))
	limitRaw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("limit")))
	wantAll = allRaw == "1" || allRaw == "true" || allRaw == "yes" || limitRaw == "all"

	if v := strings.TrimSpace(r.URL.Query().Get("page")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if !wantAll && limitRaw != "" {
		if n, err := strconv.Atoi(limitRaw); err == nil && n > 0 {
			limit = n
		}
	}
	if !wantAll && limit > a.Cfg.MaxPageSize {
		limit = a.Cfg.MaxPageSize
	}
	return page, limit, wantAll
}

func (a *API) resolvePageWindow(w http.ResponseWriter, page, limit int, wantAll bool, total int64) (int, int, int, bool) {
	if wantAll {
		if total > int64(a.Cfg.MaxAllPageSize) {
			response.Fail(w, http.StatusBadRequest, "too_many_results",
				"result set exceeds MAX_ALL_PAGE_SIZE; use page/limit pagination instead")
			return 0, 0, 0, false
		}
		if total == 0 {
			return 1, a.Cfg.DefaultPageSize, 0, true
		}
		return 1, int(total), 0, true
	}
	return page, limit, (page - 1) * limit, true
}

func pageMeta(page, limit int, total int64, wantAll bool) response.PageMeta {
	totalPages := 0
	if wantAll {
		if total == 0 {
			totalPages = 0
		} else {
			totalPages = 1
		}
	} else if limit > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}
	return response.PageMeta{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
		All:        wantAll,
	}
}
