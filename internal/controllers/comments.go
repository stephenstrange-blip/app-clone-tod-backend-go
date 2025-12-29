package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	customUtil "github.com/app-clone-tod-utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Comment struct {
	ID        int       `json:"id,omitzero"`
	Path      string    `json:"path,omitzero"`
	Depth     int       `json:"depth,omitzero"`
	NumChild  int       `json:"numChild,omitzero"`
	UpdatedAt time.Time `json:"updatedAt,omitzero"`
	Message   string    `json:"message,omitzero"`
	PostID    int       `json:"postID,omitzero"`
	AuthorID  int       `json:"authorID,omitzero"`
	IsDeleted bool      `json:"isDeleted,omitzero"`
	CreatedAt time.Time `json:"createdAt,omitzero"`
}
type CommentResponse struct {
	Err     error      `json:"err,omitzero"`
	Message string     `json:"message,omitzero"`
	Result  []*Comment `json:"result,omitzero"`
}

type CommentRequest struct {
	PostID     int    `json:"postID,omitzero"`
	CommentID  int    `json:"commentID,omitzero"`
	AuthorID   int    `json:"authorID,omitzero"`
	GetReplies bool   `json:"getReplies,omitzero"`
	Message    string `json:"message,omitzero"`
}

func (c *Controller) Comment(pool *pgxpool.Pool) http.HandlerFunc {

	return func(wr http.ResponseWriter, r *http.Request) {
		var response *CommentResponse
		var err error

		method := r.Method
		if method == "" {
			method = "GET"
		}

		// Parse path and query parameters
		params := &CommentRequest{}
		if err := params.Parse(r); err != nil {
			fmt.Printf("error (params): %s", err.Error())
			wr.WriteHeader(http.StatusBadRequest)
			return
		}

		// Handle request based on method
		switch method {
		case http.MethodGet:
			response, err = params.GetComment(pool, r.Context())
		case http.MethodDelete:
			response, err = params.DelComment(pool, r.Context())
		case http.MethodPost:
			response, err = params.PostReply(pool, r.Context())
		default:
			wr.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if err != nil {
			fmt.Printf("error (internal): %s", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Prepare output message for client
		p, err := json.Marshal(response)
		if err != nil {
			fmt.Printf("error (internal): %s", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		wr.Write(p)
	}
}

// --------------------- Service Layer -------------------------- //

func (c *CommentRequest) GetComment(p *pgxpool.Pool, ctx context.Context) (*CommentResponse, error) {
	response := &CommentResponse{}

	if c.PostID == 0 || c.CommentID == 0 {
		return nil, errors.New("bad request body")
	}

	if err := response.FetchComment(p, ctx, c.CommentID, c.PostID, c.GetReplies); err != nil {
		return nil, err
	}

	// Retrieve all subcomments under a top-level comment if client requests
	// c.Result should now contain the top-level comment as the first element, followed by its sub-comments
	if c.GetReplies {
		if err := response.FetchReply(p, ctx, c.CommentID, c.PostID); err != nil {
			return nil, err
		}
	}
	return response, nil
}

func (c *CommentRequest) DelComment(p *pgxpool.Pool, ctx context.Context) (*CommentResponse, error) {
	response := &CommentResponse{}

	if c.PostID == 0 || c.CommentID == 0 {
		return nil, errors.New("bad request body")
	}

	if err := response.RemoveComment(p, ctx, c.CommentID, c.PostID); err != nil {
		return nil, err
	}

	return response, nil
}

// Used in DynamicPostController() at posts.go
func (c *PostRequest) PostComment(p *pgxpool.Pool, ctx context.Context) (*CommentResponse, error) {
	response := &CommentResponse{}

	c.Message = strings.TrimSpace(c.Message)
	if c.Message == "" {
		return nil, errors.New("message is empty")
	}

	if c.PostID == 0 || c.AuthorID == 0 {
		return nil, errors.New("bad request body")
	}

	// Find the latest top-level comment to a post (aka not a subcomment)
	rootPath, rootErr := findLastRootComment(p, ctx)
	var pathErr error

	switch rootErr {
	case pgx.ErrNoRows:
		// Start a root path of "0001" if no pre-existing top-level comments
		rootPath, pathErr = customUtil.ConvertIntToPath(1)
	case nil:
		// increment by 1
		rootPath, pathErr = customUtil.IncrementPath(rootPath)
	default:
		// return unexpected root errors
		return nil, rootErr
	}

	if pathErr != nil {
		return nil, pathErr
	}

	if dbErr := response.CreateComment(p, ctx, c.PostID, c.AuthorID, c.Message, rootPath); dbErr != nil {
		return nil, dbErr
	}

	return response, nil
}

func (c *CommentRequest) PostReply(p *pgxpool.Pool, ctx context.Context) (*CommentResponse, error) {
	response := &CommentResponse{}
	var newPath string

	message := strings.TrimSpace(c.Message)
	if message == "" {
		return nil, errors.New("message is empty")
	}

	if c.AuthorID == 0 || c.CommentID == 0 {
		return nil, errors.New("bad request body")
	}

	// Get the root path as base to reply path, depth and numChild to increment by 1
	root, err := findRootPathBaseNumChild(p, ctx, c.CommentID, c.PostID)
	if err != nil {
		return nil, err
	}

	// Start a path if no subcomments
	if root.NumChild == 0 {
		newPath, err = customUtil.ConvertIntToPath(1)
		if err != nil {
			return nil, err
		}

		// append "0001" to root path
		newPath = root.Path + newPath

	} else {
		newPath, err = findLastChildComment(p, ctx, root.Depth, root.NumChild, root.Path)
		if err != nil {
			return nil, err
		}

		// newPath already consists of parent path
		newPath, err = customUtil.IncrementPath(newPath)
		if err != nil {
			return nil, err
		}

	}

	// Update the numChild of the root comment
	incrementNumChild, err := updateNumChild(p, ctx, root.NumChild+1, c.CommentID, root.Path)
	if err != nil {
		return nil, err
	}

	if incrementNumChild != root.NumChild+1 {
		return nil, errors.New("root numchild not updated")
	}

	if err := response.CreateReply(p, ctx, c.PostID, c.AuthorID, root.Depth+1, message, newPath); err != nil {
		return nil, err
	}

	return response, nil
}

// --------------------- Repository Layer -------------------------- //

func (c *CommentResponse) FetchComment(p *pgxpool.Pool, ctx context.Context, id, postID int, getReplies bool) error {
	x := &Comment{}

	// Consider order of columns as it appears on db
	err := p.QueryRow(ctx, `SELECT * FROM "Comment" WHERE id = $1 AND "postId" = $2`, id, postID).Scan(
		&x.ID,
		&x.Path,
		&x.Depth,
		&x.NumChild,
		nil, // ignore some column
		&x.UpdatedAt,
		&x.Message,
		&x.PostID,
		&x.AuthorID,
		&x.IsDeleted,
	)

	if err != nil {
		return err
	}

	// logger := customUtil.NewCustomLogger()

	// logger.Info("Get",
	// 	slog.Group("Comment",
	// 		slog.Int("id", x.ID),
	// 		slog.String("path", x.Path),
	// 		slog.Int("depth", x.Depth),
	// 		slog.Int("postID", x.PostID),
	// 		slog.Int("numChild", x.NumChild),
	// 		slog.Int("authorID", x.AuthorID),
	// 		slog.Bool("isDeleted", x.IsDeleted),
	// 		slog.Time("updatedAt", x.UpdatedAt),
	// 		slog.String("message", x.Message),
	// 	),
	// )

	c.Result = []*Comment{x}
	c.Err = nil
	c.Message = "Done!"

	return nil
}

func (c *CommentResponse) RemoveComment(p *pgxpool.Pool, ctx context.Context, id, postID int) error {
	x := &Comment{}

	// Does not delete a comment, only marks it deleted
	err := p.QueryRow(ctx, `UPDATE "Comment" SET "isDeleted"=$1 WHERE id=$2 AND "postId"=$3 RETURNING *`, true, id, postID).Scan(
		&x.ID,
		&x.Path,
		&x.Depth,
		&x.NumChild,
		nil, // ignore some column
		&x.UpdatedAt,
		&x.Message,
		&x.PostID,
		&x.AuthorID,
		&x.IsDeleted,
	)

	if err != nil {
		return err
	}

	c.Err = nil
	c.Message = "Done!"
	c.Result = []*Comment{x}

	return nil
}

func (c *CommentResponse) FetchReply(p *pgxpool.Pool, ctx context.Context, id int, postID int) error {
	root := &Comment{}

	// Get top-level comment first
	err := p.QueryRow(ctx, `SELECT path, depth, numchild FROM "Comment" WHERE id = $1 AND "postId" = $2`, id, postID).Scan(&root.Path, &root.Depth, &root.NumChild)
	if err != nil {
		return err
	}

	// Return if no subcomments
	if root.NumChild == 0 {
		return nil
	}

	// 4-character path ranges from "0000" to "ZZZZ"
	gtPath := root.Path + customUtil.MinSegment
	ltePath := root.Path + customUtil.MaxSegment

	// Get all comments under the top-level comment (aka whose depth is greater and has root path of that top-level comment)
	rows, _ := p.Query(ctx, `SELECT * FROM "Comment" WHERE depth > $1 AND "postId" = $2 AND path BETWEEN $3 AND $4 ORDER BY path`, root.Depth, postID, gtPath, ltePath)

	result, err := pgx.CollectRows(rows, scanComment)
	if err != nil {
		return err
	}

	// Append for cases where top-level comment is already fetched
	if c.Result != nil {
		c.Result = append(c.Result, result...)
	} else {
		c.Result = result
	}
	c.Err = nil
	c.Message = "Done!"

	return nil
}

func (c *CommentResponse) CreateComment(p *pgxpool.Pool, ctx context.Context, postID, authorID int, message, rootPath string) error {
	var (
		comment   = &Comment{}
		createdAt = time.Now()
	)

	// Insert a new row and fetch the autoincremented id
	err := p.QueryRow(
		ctx,
		`INSERT INTO "Comment" ("path", "depth", "createdAt", "updatedAt", "message", "postId", "authorId") VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		rootPath, 1, createdAt, createdAt, message, postID, authorID,
	).Scan(&comment.ID)

	if err != nil {
		return err
	}

	c.Result = []*Comment{comment}
	c.Err = nil
	c.Message = "Done!"

	return nil
}

func (c *CommentResponse) CreateReply(p *pgxpool.Pool, ctx context.Context, postID, authorID, depth int, message, path string) error {
	var (
		err   error
		reply = &Comment{}

		createdAt = time.Now()
	)

	// Insert a new row and fetch the autoincremented id
	err = p.QueryRow(
		ctx,
		`INSERT INTO "Comment" ("path", "depth", "createdAt", "updatedAt", "message", "postId", "authorId") VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		path, depth, createdAt, createdAt, message, postID, authorID,
	).Scan(&reply.ID)

	if err != nil {
		return err
	}

	c.Result = []*Comment{reply}
	c.Err = nil
	c.Message = "Done!"
	return nil
}

// Used at posts.go
func GetCommentCount(p *pgxpool.Pool, ctx context.Context, postID int) (int, error) {
	var count int
	err := p.QueryRow(ctx, `SELECT COUNT(*) FROM "Comment" WHERE "postId" = $1`, postID).Scan(&count)

	return count, err
}

func (c *CommentRequest) Parse(r *http.Request) error {
	// Empty strings is checked in each method handler
	if r.Method != http.MethodGet {
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			return err
		}
	}

	// Required int query params
	postID, err := strconv.ParseInt(r.PathValue("postID"), 10, 0)
	if err != nil {
		return err
	}
	c.PostID = int(postID)

	commentID, err := strconv.ParseInt(r.PathValue("commentID"), 10, 0)
	if err != nil {
		return err
	}
	c.CommentID = int(commentID)

	userID, ok := UserFromContext(r.Context())
	if !ok {
		return errors.New("userID not found")
	}
	c.AuthorID = userID

	// Optional bool form value
	if shouldGetReplies := r.FormValue("getReplies"); shouldGetReplies != "" {
		getReplies, err := strconv.ParseBool(shouldGetReplies)
		if err != nil {
			return err
		}
		c.GetReplies = getReplies
	}

	return nil
}

// --------------------- Utility Layer -------------------------- //

func findLastChildComment(p *pgxpool.Pool, ctx context.Context, depth, numChild int, path string) (string, error) {
	var childPath string

	// 4-character path ranges from "0000" to "ZZZZ"
	gtPath := path + customUtil.MinSegment
	ltePath := path + customUtil.MaxSegment

	err := p.QueryRow(ctx, `SELECT path FROM "Comment" WHERE depth > $1 AND path BETWEEN $2 AND $3 ORDER BY path DESC`, depth, gtPath, ltePath).Scan(&childPath)
	if err != nil && errors.Is(err, pgx.ErrNoRows) && numChild > 0 {
		return "", errors.New("cannot retrieve subcomments despite root.numchild > 0")
	} else if err != nil {
		return "", err
	}

	return childPath, nil
}

func findRootPathBaseNumChild(p *pgxpool.Pool, ctx context.Context, id, postID int) (*Comment, error) {
	root := &Comment{}

	err := p.QueryRow(ctx, `SELECT path, depth, numchild FROM "Comment" WHERE id = $1 AND "postId" = $2`, id, postID).Scan(&root.Path, &root.Depth, &root.NumChild)
	if err != nil {
		return nil, err
	}

	return root, nil
}

func updateNumChild(p *pgxpool.Pool, ctx context.Context, numChild, id int, path string) (int, error) {
	var incrementNumChild int
	err := p.QueryRow(ctx, `UPDATE "Comment" SET numchild = $1 WHERE id = $2 AND path = $3 RETURNING numchild`, numChild, id, path).Scan(&incrementNumChild)
	if err != nil {
		return 0, err
	}
	return incrementNumChild, nil
}

func findLastRootComment(p *pgxpool.Pool, ctx context.Context) (string, error) {
	var path string

	// top-level comments have depth of 1
	err := p.QueryRow(ctx, `SELECT path FROM "Comment" WHERE depth = 1 ORDER BY path DESC`).Scan(&path)
	if err != nil {
		return "", err
	}

	return path, nil
}

func scanComment(row pgx.CollectableRow) (*Comment, error) {
	x := &Comment{}

	// Consider order of columns as it appears on db
	err := row.Scan(
		&x.ID,
		&x.Path,
		&x.Depth,
		&x.NumChild,
		nil, // ignore a column
		&x.UpdatedAt,
		&x.Message,
		&x.PostID,
		&x.AuthorID,
		&x.IsDeleted,
	)
	if err != nil {
		return x, err
	}

	return x, nil
}
