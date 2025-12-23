package entity

// CommentStatistics represents aggregated comment statistics for an account
type CommentStatistics struct {
	TotalComments      int64     `json:"total_comments"`        // Total count of comments
	RepliedComments    int64     `json:"replied_comments"`      // Count of replies from account
	AvgCommentsPerPost float64   `json:"avg_comments_per_post"` // Average comments per post
	TopPosts           []TopPost `json:"top_posts"`             // Top posts by comment count
}

// TopPost represents a post with its comment count
type TopPost struct {
	MediaID       string `json:"media_id"`
	Caption       string `json:"caption,omitempty"`
	Thumbnail     string `json:"thumbnail,omitempty"`
	CommentsCount int64  `json:"comments_count"`
}
