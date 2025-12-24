package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vadim/neo-metric/internal/domain/comment/entity"
)

// CommentRepository defines the interface for comment storage
type CommentRepository interface {
	// Upsert inserts or updates a comment
	Upsert(ctx context.Context, comment *entity.Comment) error
	// UpsertBatch inserts or updates multiple comments
	UpsertBatch(ctx context.Context, comments []entity.Comment) error
	// GetByID retrieves a comment by ID
	GetByID(ctx context.Context, id string) (*entity.Comment, error)
	// GetByMediaID retrieves comments for a media
	GetByMediaID(ctx context.Context, mediaID string, limit int, offset int) ([]entity.Comment, error)
	// GetReplies retrieves replies to a comment
	GetReplies(ctx context.Context, parentID string, limit int, offset int) ([]entity.Comment, error)
	// Delete removes a comment
	Delete(ctx context.Context, id string) error
	// UpdateHidden updates the hidden status
	UpdateHidden(ctx context.Context, id string, hidden bool) error
	// Count returns the total count of comments for a media
	Count(ctx context.Context, mediaID string) (int64, error)
	// CountReplies returns the total count of replies to a comment
	CountReplies(ctx context.Context, parentID string) (int64, error)
	// GetStatistics retrieves aggregated comment statistics for an account
	GetStatistics(ctx context.Context, accountID string, topPostsLimit int) (*entity.CommentStatistics, error)
}

// SyncStatusRepository defines the interface for sync status tracking
type SyncStatusRepository interface {
	// GetSyncStatus retrieves sync status for a media
	GetSyncStatus(ctx context.Context, mediaID string) (*SyncStatus, error)
	// UpdateSyncStatus updates sync status for a media
	UpdateSyncStatus(ctx context.Context, status *SyncStatus) error
	// GetMediaIDsNeedingSync retrieves media IDs that need synchronization
	GetMediaIDsNeedingSync(ctx context.Context, olderThan time.Duration, limit int) ([]string, error)
	// IncrementRetryCount increments the retry count and optionally marks as failed
	IncrementRetryCount(ctx context.Context, mediaID string, lastError string, maxRetries int) error
	// ResetRetryCount resets the retry count after a successful sync
	ResetRetryCount(ctx context.Context, mediaID string) error
}

// SyncStatus represents the synchronization status for a media's comments
type SyncStatus struct {
	InstagramMediaID string
	LastSyncedAt     time.Time
	NextCursor       string
	SyncComplete     bool
	RetryCount       int
	Failed           bool
	LastError        string
}

// CommentPostgres implements CommentRepository for PostgreSQL
type CommentPostgres struct {
	pool *pgxpool.Pool
}

// NewCommentPostgres creates a new PostgreSQL comment repository
func NewCommentPostgres(pool *pgxpool.Pool) *CommentPostgres {
	return &CommentPostgres{pool: pool}
}

// Upsert inserts or updates a comment
func (r *CommentPostgres) Upsert(ctx context.Context, comment *entity.Comment) error {
	query := `
		INSERT INTO comments (id, instagram_media_id, parent_id, author_id, username, text, like_count, is_hidden, timestamp, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (id) DO UPDATE SET
			like_count = EXCLUDED.like_count,
			is_hidden = EXCLUDED.is_hidden,
			text = EXCLUDED.text,
			author_id = COALESCE(EXCLUDED.author_id, comments.author_id),
			updated_at = NOW()
	`

	var parentID *string
	if comment.ParentID != "" {
		parentID = &comment.ParentID
	}

	var authorID *string
	if comment.AuthorID != "" {
		authorID = &comment.AuthorID
	}

	_, err := r.pool.Exec(ctx, query,
		comment.ID,
		comment.MediaID,
		parentID,
		authorID,
		comment.Username,
		comment.Text,
		comment.LikeCount,
		comment.IsHidden,
		comment.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("upserting comment: %w", err)
	}

	return nil
}

// UpsertBatch inserts or updates multiple comments
func (r *CommentPostgres) UpsertBatch(ctx context.Context, comments []entity.Comment) error {
	if len(comments) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	query := `
		INSERT INTO comments (id, instagram_media_id, parent_id, author_id, username, text, like_count, is_hidden, timestamp, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (id) DO UPDATE SET
			like_count = EXCLUDED.like_count,
			is_hidden = EXCLUDED.is_hidden,
			text = EXCLUDED.text,
			author_id = COALESCE(EXCLUDED.author_id, comments.author_id),
			updated_at = NOW()
	`

	for _, comment := range comments {
		var parentID *string
		if comment.ParentID != "" {
			parentID = &comment.ParentID
		}
		var authorID *string
		if comment.AuthorID != "" {
			authorID = &comment.AuthorID
		}
		batch.Queue(query,
			comment.ID,
			comment.MediaID,
			parentID,
			authorID,
			comment.Username,
			comment.Text,
			comment.LikeCount,
			comment.IsHidden,
			comment.Timestamp,
		)
	}

	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(comments); i++ {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("upserting comment %d: %w", i, err)
		}
	}

	return nil
}

// GetByID retrieves a comment by ID
func (r *CommentPostgres) GetByID(ctx context.Context, id string) (*entity.Comment, error) {
	query := `
		SELECT id, instagram_media_id, parent_id, author_id, username, text, like_count, is_hidden, timestamp
		FROM comments
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)

	var comment entity.Comment
	var parentID, authorID *string

	err := row.Scan(
		&comment.ID,
		&comment.MediaID,
		&parentID,
		&authorID,
		&comment.Username,
		&comment.Text,
		&comment.LikeCount,
		&comment.IsHidden,
		&comment.Timestamp,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning comment: %w", err)
	}

	if parentID != nil {
		comment.ParentID = *parentID
	}
	if authorID != nil {
		comment.AuthorID = *authorID
	}

	return &comment, nil
}

// GetByMediaID retrieves comments for a media (excluding replies)
func (r *CommentPostgres) GetByMediaID(ctx context.Context, mediaID string, limit int, offset int) ([]entity.Comment, error) {
	query := `
		SELECT id, instagram_media_id, parent_id, author_id, username, text, like_count, is_hidden, timestamp,
		       (SELECT COUNT(*) FROM comments c2 WHERE c2.parent_id = comments.id) as replies_count
		FROM comments
		WHERE instagram_media_id = $1 AND parent_id IS NULL
		ORDER BY timestamp DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, mediaID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying comments: %w", err)
	}
	defer rows.Close()

	var comments []entity.Comment
	for rows.Next() {
		var comment entity.Comment
		var parentID, authorID *string

		err := rows.Scan(
			&comment.ID,
			&comment.MediaID,
			&parentID,
			&authorID,
			&comment.Username,
			&comment.Text,
			&comment.LikeCount,
			&comment.IsHidden,
			&comment.Timestamp,
			&comment.RepliesCount,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		if parentID != nil {
			comment.ParentID = *parentID
		}
		if authorID != nil {
			comment.AuthorID = *authorID
		}

		comments = append(comments, comment)
	}

	return comments, nil
}

// GetReplies retrieves replies to a comment
func (r *CommentPostgres) GetReplies(ctx context.Context, parentID string, limit int, offset int) ([]entity.Comment, error) {
	query := `
		SELECT id, instagram_media_id, parent_id, author_id, username, text, like_count, is_hidden, timestamp
		FROM comments
		WHERE parent_id = $1
		ORDER BY timestamp ASC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, parentID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying replies: %w", err)
	}
	defer rows.Close()

	var comments []entity.Comment
	for rows.Next() {
		var comment entity.Comment
		var pID, authorID *string

		err := rows.Scan(
			&comment.ID,
			&comment.MediaID,
			&pID,
			&authorID,
			&comment.Username,
			&comment.Text,
			&comment.LikeCount,
			&comment.IsHidden,
			&comment.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		if pID != nil {
			comment.ParentID = *pID
		}
		if authorID != nil {
			comment.AuthorID = *authorID
		}

		comments = append(comments, comment)
	}

	return comments, nil
}

// Delete removes a comment
func (r *CommentPostgres) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM comments WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting comment: %w", err)
	}
	return nil
}

// UpdateHidden updates the hidden status
func (r *CommentPostgres) UpdateHidden(ctx context.Context, id string, hidden bool) error {
	query := "UPDATE comments SET is_hidden = $2, updated_at = NOW() WHERE id = $1"
	_, err := r.pool.Exec(ctx, query, id, hidden)
	if err != nil {
		return fmt.Errorf("updating hidden status: %w", err)
	}
	return nil
}

// Count returns the total count of comments for a media (excluding replies)
func (r *CommentPostgres) Count(ctx context.Context, mediaID string) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM comments WHERE instagram_media_id = $1 AND parent_id IS NULL",
		mediaID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting comments: %w", err)
	}
	return count, nil
}

// CountReplies returns the total count of replies to a comment
func (r *CommentPostgres) CountReplies(ctx context.Context, parentID string) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM comments WHERE parent_id = $1",
		parentID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting replies: %w", err)
	}
	return count, nil
}

// SyncStatusPostgres implements SyncStatusRepository for PostgreSQL
type SyncStatusPostgres struct {
	pool *pgxpool.Pool
}

// NewSyncStatusPostgres creates a new PostgreSQL sync status repository
func NewSyncStatusPostgres(pool *pgxpool.Pool) *SyncStatusPostgres {
	return &SyncStatusPostgres{pool: pool}
}

// GetSyncStatus retrieves sync status for a media
func (r *SyncStatusPostgres) GetSyncStatus(ctx context.Context, mediaID string) (*SyncStatus, error) {
	query := `
		SELECT instagram_media_id, last_synced_at, next_cursor, sync_complete,
		       COALESCE(retry_count, 0), COALESCE(failed, false), COALESCE(last_error, '')
		FROM comment_sync_status
		WHERE instagram_media_id = $1
	`

	row := r.pool.QueryRow(ctx, query, mediaID)

	var status SyncStatus
	var nextCursor *string

	err := row.Scan(
		&status.InstagramMediaID,
		&status.LastSyncedAt,
		&nextCursor,
		&status.SyncComplete,
		&status.RetryCount,
		&status.Failed,
		&status.LastError,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning sync status: %w", err)
	}

	if nextCursor != nil {
		status.NextCursor = *nextCursor
	}

	return &status, nil
}

// UpdateSyncStatus updates sync status for a media
func (r *SyncStatusPostgres) UpdateSyncStatus(ctx context.Context, status *SyncStatus) error {
	query := `
		INSERT INTO comment_sync_status (instagram_media_id, last_synced_at, next_cursor, sync_complete)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (instagram_media_id) DO UPDATE SET
			last_synced_at = EXCLUDED.last_synced_at,
			next_cursor = EXCLUDED.next_cursor,
			sync_complete = EXCLUDED.sync_complete
	`

	var nextCursor *string
	if status.NextCursor != "" {
		nextCursor = &status.NextCursor
	}

	_, err := r.pool.Exec(ctx, query,
		status.InstagramMediaID,
		status.LastSyncedAt,
		nextCursor,
		status.SyncComplete,
	)
	if err != nil {
		return fmt.Errorf("updating sync status: %w", err)
	}

	return nil
}

// GetMediaIDsNeedingSync retrieves media IDs that need synchronization
// Note: Stories are excluded because Instagram API doesn't support comments endpoint for them
// Media marked as failed are excluded from sync
func (r *SyncStatusPostgres) GetMediaIDsNeedingSync(ctx context.Context, olderThan time.Duration, limit int) ([]string, error) {
	query := `
		SELECT p.instagram_media_id
		FROM publications p
		LEFT JOIN comment_sync_status css ON p.instagram_media_id = css.instagram_media_id
		WHERE p.instagram_media_id IS NOT NULL
		  AND p.status = 'published'
		  AND p.type != 'story'
		  AND (css.failed IS NULL OR css.failed = false)
		  AND (css.last_synced_at IS NULL OR css.last_synced_at < $1)
		ORDER BY COALESCE(css.last_synced_at, '1970-01-01'::timestamp) ASC
		LIMIT $2
	`

	cutoff := time.Now().Add(-olderThan)
	rows, err := r.pool.Query(ctx, query, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("querying media ids: %w", err)
	}
	defer rows.Close()

	var mediaIDs []string
	for rows.Next() {
		var mediaID string
		if err := rows.Scan(&mediaID); err != nil {
			return nil, fmt.Errorf("scanning media id: %w", err)
		}
		mediaIDs = append(mediaIDs, mediaID)
	}

	return mediaIDs, nil
}

// IncrementRetryCount increments the retry count and marks as failed if max retries exceeded
func (r *SyncStatusPostgres) IncrementRetryCount(ctx context.Context, mediaID string, lastError string, maxRetries int) error {
	query := `
		INSERT INTO comment_sync_status (instagram_media_id, last_synced_at, retry_count, last_error, failed)
		VALUES ($1, NOW(), 1, $2, 1 >= $3)
		ON CONFLICT (instagram_media_id) DO UPDATE SET
			retry_count = comment_sync_status.retry_count + 1,
			last_error = EXCLUDED.last_error,
			failed = (comment_sync_status.retry_count + 1) >= $3,
			last_synced_at = NOW()
	`

	_, err := r.pool.Exec(ctx, query, mediaID, lastError, maxRetries)
	if err != nil {
		return fmt.Errorf("incrementing retry count: %w", err)
	}

	return nil
}

// ResetRetryCount resets the retry count after a successful sync
func (r *SyncStatusPostgres) ResetRetryCount(ctx context.Context, mediaID string) error {
	query := `
		UPDATE comment_sync_status
		SET retry_count = 0, failed = false, last_error = NULL
		WHERE instagram_media_id = $1
	`

	_, err := r.pool.Exec(ctx, query, mediaID)
	if err != nil {
		return fmt.Errorf("resetting retry count: %w", err)
	}

	return nil
}

// GetStatistics retrieves aggregated comment statistics for an account
func (r *CommentPostgres) GetStatistics(ctx context.Context, accountID string, topPostsLimit int) (*entity.CommentStatistics, error) {
	stats := &entity.CommentStatistics{}

	// Get total comments count for account's publications
	totalQuery := `
		SELECT COUNT(*)
		FROM comments c
		JOIN publications p ON p.instagram_media_id = c.instagram_media_id
		WHERE p.account_id = $1 AND p.status = 'published'
	`
	if err := r.pool.QueryRow(ctx, totalQuery, accountID).Scan(&stats.TotalComments); err != nil {
		return nil, fmt.Errorf("counting total comments: %w", err)
	}

	// Get replied comments count (replies made by account owner)
	// Count replies where:
	// 1. username matches account owner's username, OR
	// 2. comment has a parent_id pointing to a comment on the account's publications
	//    (for cases where username is empty but it's still a reply from the owner)
	repliedQuery := `
		SELECT COUNT(*)
		FROM comments c
		JOIN instagram_accounts ia ON ia.id = $1
		WHERE (
			-- Case 1: Direct match by username on account's publications
			(c.instagram_media_id IN (
				SELECT instagram_media_id FROM publications
				WHERE account_id = $1 AND status = 'published'
			) AND c.username = ia.username)
			OR
			-- Case 2: Reply to a comment on account's publications (parent_id based)
			(c.parent_id IS NOT NULL AND c.parent_id IN (
				SELECT c2.id FROM comments c2
				JOIN publications p ON p.instagram_media_id = c2.instagram_media_id
				WHERE p.account_id = $1 AND p.status = 'published'
			) AND (c.username = ia.username OR c.username = '' OR c.username IS NULL))
		)
	`
	if err := r.pool.QueryRow(ctx, repliedQuery, accountID).Scan(&stats.RepliedComments); err != nil {
		return nil, fmt.Errorf("counting replied comments: %w", err)
	}

	// Get average comments per post
	avgQuery := `
		SELECT COALESCE(AVG(comment_count), 0)
		FROM (
			SELECT COUNT(*) as comment_count
			FROM publications p
			LEFT JOIN comments c ON c.instagram_media_id = p.instagram_media_id
			WHERE p.account_id = $1
			  AND p.status = 'published'
			  AND p.instagram_media_id IS NOT NULL
			GROUP BY p.id
		) subq
	`
	if err := r.pool.QueryRow(ctx, avgQuery, accountID).Scan(&stats.AvgCommentsPerPost); err != nil {
		return nil, fmt.Errorf("calculating avg comments: %w", err)
	}

	// Get top posts by comment count
	if topPostsLimit <= 0 {
		topPostsLimit = 5
	}
	topQuery := `
		SELECT p.instagram_media_id, COALESCE(p.caption, ''), COUNT(c.id) as comments_count
		FROM publications p
		LEFT JOIN comments c ON c.instagram_media_id = p.instagram_media_id
		WHERE p.account_id = $1
		  AND p.status = 'published'
		  AND p.instagram_media_id IS NOT NULL
		GROUP BY p.id, p.instagram_media_id, p.caption
		ORDER BY comments_count DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, topQuery, accountID, topPostsLimit)
	if err != nil {
		return nil, fmt.Errorf("querying top posts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var post entity.TopPost
		if err := rows.Scan(&post.MediaID, &post.Caption, &post.CommentsCount); err != nil {
			return nil, fmt.Errorf("scanning top post: %w", err)
		}
		stats.TopPosts = append(stats.TopPosts, post)
	}

	return stats, nil
}
