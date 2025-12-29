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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostRequest struct {
	PostID     int       `json:"postID,omitzero"`
	AuthorID   int       `json:"authorID,omitzero"`
	Message    string    `json:"message,omitzero"`
	Published  *bool     `json:"published,omitempty"`
	MyPosts    bool      `json:"myPosts,omitempty"`
	CategoryID int       `json:"categoryID,omitzero"`
	Title      string    `json:"title,omitzero"`
	Start      time.Time `json:"start,omitzero"`
	End        time.Time `json:"end,omitzero"`
}

type PostResponse struct {
	Err     error   `json:"err,omitzero"`
	Message string  `json:"message,omitzero"`
	Result  []*Post `json:"result"`
}

type Post struct {
	Id         int       `json:"id,omitzero"`
	Title      string    `json:"title,omitzero"`
	Message    string    `json:"message,omitzero"`
	CreatedAt  time.Time `json:"createdAt,omitzero"`
	UpdatedAt  time.Time `json:"updatedAt,omitzero"`
	CategoryID int       `json:"categoryID,omitzero"`
	IsDeleted  bool      `json:"isDeleted,omitzero"`
	Published  bool      `json:"published,omitzero"` // is false automatically when missing from response result
	Author     Author    `json:"author,omitzero"`

	Count struct {
		Reactions int `json:"reactions,omitempty"`
		Comments  int `json:"comments,omitempty"`
	} `json:"_count,omitzero"`

	Reactions []*Reaction `json:"reactions,omitzero"`
}

func (c *Controller) BasePostRoute(pool *pgxpool.Pool) http.HandlerFunc {

	return func(wr http.ResponseWriter, r *http.Request) {
		var response *PostResponse
		var err error

		method := r.Method
		if method == "" {
			method = "GET"
		}

		params := &PostRequest{}

		if err := params.Parse(r); err != nil {
			fmt.Printf("error (params): %s\n", err.Error())
			wr.WriteHeader(http.StatusBadRequest)
			return
		}

		switch method {
		case http.MethodGet:
			response, err = params.GetPosts(pool, r.Context())
		case http.MethodPost:
			response, err = params.PostPost(pool, r.Context())
		default:
			wr.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if err != nil {
			fmt.Printf("error (internal): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		if p, err := json.Marshal(response); err != nil {
			fmt.Printf("error (internal): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		} else {
			wr.Write(p)
		}

	}
}

func (c *Controller) DynamicPostRoute(pool *pgxpool.Pool) http.HandlerFunc {

	return func(wr http.ResponseWriter, r *http.Request) {
		var (
			response        *PostResponse
			commentResponse *CommentResponse
			p               []byte
			err             error
		)

		method := r.Method
		if method == "" {
			method = "GET"
		}

		params := &PostRequest{}

		if err := params.Parse(r); err != nil {
			fmt.Printf("error (params): %s\n", err.Error())
			wr.WriteHeader(http.StatusBadRequest)
			return
		}

		switch method {
		case http.MethodGet:
			response, err = params.GetPost(pool, r.Context())
		case http.MethodPost:
			// Handles top-level comment creation for a post
			commentResponse, err = params.PostComment(pool, r.Context())
		case http.MethodPut:
			response, err = params.PutPost(pool, r.Context())
		case http.MethodDelete:
			response, err = params.DelPost(pool, r.Context())
		default:
			wr.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if err != nil {
			fmt.Printf("error (internal): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		if method == http.MethodPost {
			p, err = json.Marshal(commentResponse)
		} else {
			p, err = json.Marshal(response)
		}

		if err != nil {
			fmt.Printf("error (internal): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		} else {
			wr.Write(p)
		}

	}
}

// --------------------- Service Layer -------------------------- //

func (pr *PostRequest) DelPost(p *pgxpool.Pool, ctx context.Context) (*PostResponse, error) {
	response := &PostResponse{}

	if pr.PostID == 0 {
		return nil, errors.New("bad request body")
	}

	if err := response.RemovePost(p, ctx, pr.PostID); err != nil {
		return nil, err
	}

	return response, nil
}

func (pr *PostRequest) GetPost(p *pgxpool.Pool, ctx context.Context) (*PostResponse, error) {
	response := &PostResponse{}

	if pr.PostID == 0 {
		return nil, errors.New("bad request body")
	}

	if err := response.FetchPost(p, ctx, pr.PostID); err != nil {
		return nil, err
	}

	return response, nil
}

func (pr *PostRequest) GetPosts(p *pgxpool.Pool, ctx context.Context) (*PostResponse, error) {
	response := &PostResponse{}

	// Check for zero-value fields (unset) except for AuthorID which is unset if myPosts is false
	fmt.Printf("myPosts %t,  AuthorID %d, categoryID %d, published == nil %t\n", pr.MyPosts, pr.AuthorID, pr.CategoryID, pr.Published == nil)
	if (pr.MyPosts && pr.AuthorID == 0) || pr.CategoryID == 0 || pr.Published == nil {
		return nil, errors.New("bad request body")
	}

	if pr.MyPosts {
		var dbErr error

		if pr.Start.IsZero() && pr.End.IsZero() {
			fmt.Println("Fetching all author posts")
			// Get all posts by author
			dbErr = response.FetchPostsByAuthor(p, ctx, pr.CategoryID, pr.AuthorID, *pr.Published)
		} else if !pr.Start.IsZero() && pr.End.IsZero() {
			fmt.Println("Fetching all author posts from start to present")
			// Get all posts by author from provided start time to present
			dbErr = response.FetchPostsByAuthorBetween(p, ctx, pr.CategoryID, pr.AuthorID, *pr.Published, pr.Start, time.Now())
		} else if !pr.Start.IsZero() && !pr.End.IsZero() {
			fmt.Println("Fetching all author posts from date range")
			// Get all posts by authror from provided date range
			dbErr = response.FetchPostsByAuthorBetween(p, ctx, pr.CategoryID, pr.AuthorID, *pr.Published, pr.Start, pr.End)
		}

		if dbErr != nil {
			return nil, dbErr
		}
	} else {
		var dbErr error

		if pr.Start.IsZero() && pr.End.IsZero() {
			fmt.Println("Fetching all posts")
			// Get all posts
			dbErr = response.CreatePosts(p, ctx, pr.CategoryID, *pr.Published)
		} else if !pr.Start.IsZero() && pr.End.IsZero() {
			fmt.Println("Fetching all posts from start to present")
			// Get all posts from provided start time to present
			dbErr = response.FetchPostsBetween(p, ctx, pr.CategoryID, *pr.Published, pr.Start, time.Now())
		} else if !pr.Start.IsZero() && !pr.End.IsZero() {
			fmt.Println("Fetching all posts from date range")
			// Get all posts from provided date range
			dbErr = response.FetchPostsBetween(p, ctx, pr.CategoryID, *pr.Published, pr.Start, pr.End)
		}

		if dbErr != nil {
			return nil, dbErr
		}
	}

	return response, nil
}

func (pr *PostRequest) PutPost(p *pgxpool.Pool, ctx context.Context) (*PostResponse, error) {
	var (
		sqlArgs    = []any{}
		setClauses = []string{}
		updatedAt  = time.Now()
		argPos     = 1
		response   = &PostResponse{}
	)

	if pr.PostID == 0 {
		return nil, errors.New("bad request body")
	}

	// dynamically add each post details in db query if they are present in the request body
	if pr.Title != "" {
		sqlArgs = append(sqlArgs, pr.Title)
		setClauses = append(setClauses, fmt.Sprintf(`"title" = $%d`, argPos))
		argPos++
	}

	if pr.CategoryID != 0 {
		sqlArgs = append(sqlArgs, pr.CategoryID)
		setClauses = append(setClauses, fmt.Sprintf(`"categoryId" = $%d`, argPos))
		argPos++
	}

	if strings.TrimSpace(pr.Message) != "" {
		sqlArgs = append(sqlArgs, pr.Message)
		setClauses = append(setClauses, fmt.Sprintf(`"message" = $%d`, argPos))
		argPos++
	}

	// changed to a pointer (from using a value of bool) to use nil and indicate that client doesn't want to update published status
	if pr.Published != nil {
		sqlArgs = append(sqlArgs, pr.Published)
		setClauses = append(setClauses, fmt.Sprintf(`"published" = $%d`, argPos))
		argPos++
	}

	if len(sqlArgs) == 0 {
		return nil, errors.New("no post field to update")
	}

	// If update will proceed, indicate time of update
	sqlArgs = append(sqlArgs, updatedAt)
	setClauses = append(setClauses, fmt.Sprintf(`"updatedAt" = $%d`, argPos))
	argPos++

	query := fmt.Sprintf(`UPDATE "Post" SET %s WHERE id = %d RETURNING *`, strings.Join(setClauses, ", "), pr.PostID)

	if err := response.UpdatePost(p, ctx, query, pr.PostID, sqlArgs...); err != nil {
		return nil, err
	}

	return response, nil
}

func (pr *PostRequest) PostPost(p *pgxpool.Pool, ctx context.Context) (*PostResponse, error) {
	if pr.AuthorID == 0 {
		return nil, errors.New("bad request body")
	}

	var (
		table     = []string{`"title"`, `"createdAt"`, `"updatedAt"`, `"authorId"`}
		argPos    = []string{"$1", "$2", "$3", "$4"}
		pos       = 5
		createdAt = time.Now()
		sqlArgs   = []any{pr.Title, createdAt, createdAt, pr.AuthorID}
		response  = &PostResponse{}
	)

	message := strings.ReplaceAll(pr.Message, " ", "")

	// empty message should be left nil in db
	if message != "" {
		table = append(table, `"message"`)
		argPos = append(argPos, fmt.Sprintf("$%d", pos))
		sqlArgs = append(sqlArgs, message)
		pos++
	}

	if pr.CategoryID != 0 {
		table = append(table, `"categoryId"`)
		argPos = append(argPos, fmt.Sprintf("$%d", pos))
		sqlArgs = append(sqlArgs, pr.CategoryID)
		pos++
	}

	query := fmt.Sprintf(`INSERT INTO "Post" (%s) VALUES (%s) RETURNING "id", "authorId", "createdAt"`, strings.Join(table, ", "), strings.Join(argPos, ", "))

	if err := response.CreatePost(p, ctx, query, sqlArgs...); err != nil {
		return nil, err
	}

	return response, nil
}

// --------------------- Repository Layer -------------------------- //

// TODO: Implement soft deletes (marking as deleted)
func (pr *PostResponse) RemovePost(p *pgxpool.Pool, ctx context.Context, postID int) error {
	var (
		author   *Author
		authorID int
		x        = &Post{}
	)
	// Send back details in case of UNDO
	err := p.QueryRow(ctx, `DELETE FROM ONLY "Post" WHERE id = $1 RETURNING *`, postID).Scan(
		&x.Id,
		&x.Title,
		&x.Message,
		&x.CreatedAt,
		&x.UpdatedAt,
		&x.Published,
		&authorID,
		&x.CategoryID,
		&x.IsDeleted,
	)

	if err != nil {
		return err
	}

	// Get author details as well
	if author, err = FetchAuthor(p, ctx, authorID); err != nil {
		return err
	}

	x.Author.FirstName = author.FirstName
	x.Author.LastName = author.LastName

	pr.Err = nil
	pr.Message = "Done!"
	pr.Result = []*Post{x}

	return nil
}

func (pr *PostResponse) FetchPost(p *pgxpool.Pool, ctx context.Context, postID int) error {

	rows, _ := p.Query(context.Background(), `SELECT * FROM "Post" WHERE id = $1`, postID)

	result, err := pgx.CollectRows(rows, scanPost(p, ctx))
	if err != nil {
		return err
	}

	pr.Result = result
	pr.Err = nil
	pr.Message = "Done!"
	return nil
}

func (pr *PostResponse) UpdatePost(p *pgxpool.Pool, ctx context.Context, query string, postID int, sqlArgs ...any) error {
	updatedPost := &Post{}

	err := p.QueryRow(ctx, query, sqlArgs...).Scan(
		&updatedPost.Id,
		&updatedPost.Title,
		&updatedPost.Message,
		&updatedPost.CreatedAt,
		&updatedPost.UpdatedAt,
		&updatedPost.Published,
		nil, // skip author as this should not change
		&updatedPost.CategoryID,
		&updatedPost.IsDeleted,
	)

	if err != nil {
		return err
	}

	pr.Err = nil
	pr.Message = "Done!"
	pr.Result = []*Post{updatedPost}

	return nil
}

func (pr *PostResponse) CreatePost(p *pgxpool.Pool, ctx context.Context, query string, sqlArgs ...any) error {
	rows, _ := p.Query(ctx, query, sqlArgs...)

	// Cannot use scanPost for fresh posts (fewer return params needed)
	result, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Post, error) {
		var (
			x         = &Post{}
			author    = &Author{}
			err       error
			authorID  int
			createdAt time.Time
		)

		if err = row.Scan(&x.Id, &authorID, &createdAt); err != nil {
			return nil, err
		}

		if author, err = FetchAuthor(p, ctx, authorID); err != nil {
			return nil, err
		}

		x.Author.FirstName = author.FirstName
		x.Author.LastName = author.LastName
		x.CreatedAt = createdAt

		return x, nil
	})

	if err != nil {
		return err
	}

	pr.Err = nil
	pr.Message = "Done!"
	pr.Result = result
	return nil
}

func (pr *PostResponse) CreatePosts(p *pgxpool.Pool, ctx context.Context, categoryID int, published bool) error {
	rows, _ := p.Query(context.Background(), `SELECT * FROM  "Post" WHERE "categoryId" = $1 AND published = $2`, categoryID, published)

	result, err := pgx.CollectRows(rows, scanPost(p, ctx))
	if err != nil {
		return err
	}

	pr.Result = result
	pr.Message = "Done!"
	pr.Err = nil
	return nil
}

func (pr *PostResponse) FetchPostsBetween(p *pgxpool.Pool, ctx context.Context, categoryID int, published bool, start, end time.Time) error {
	rows, _ := p.Query(context.Background(), `SELECT * FROM  "Post" WHERE "categoryId" = $1 AND published = $2 AND "updatedAt" BETWEEN $3 AND $4 ORDER BY id`, categoryID, published, start.Format(time.RFC3339), end.Format(time.RFC3339))

	result, err := pgx.CollectRows(rows, scanPost(p, ctx))
	if err != nil {
		return err
	}

	pr.Result = result
	pr.Message = "Done!"
	pr.Err = nil

	return nil
}

func (pr *PostResponse) FetchPostsByAuthor(p *pgxpool.Pool, ctx context.Context, categoryID, authorID int, published bool) error {
	rows, _ := p.Query(context.Background(), `SELECT * FROM  "Post" WHERE "categoryId" = $1 AND published = $2 AND "authorId" = $3`, categoryID, published, authorID)

	result, err := pgx.CollectRows(rows, scanPost(p, ctx))
	if err != nil {
		return err
	}

	pr.Result = result
	pr.Message = "Done!"
	pr.Err = nil
	return nil
}

func (pr *PostResponse) FetchPostsByAuthorBetween(p *pgxpool.Pool, ctx context.Context, categoryID, authorID int, published bool, start, end time.Time) error {
	rows, _ := p.Query(ctx, `SELECT * FROM  "Post" WHERE "categoryId" = $1 AND published = $2 AND "authorId" = $3 AND "updatedAt" BETWEEN $4 AND $5 ORDER BY id`, categoryID, published, authorID, start.Format(time.RFC3339), end.Format(time.RFC3339))

	result, err := pgx.CollectRows(rows, scanPost(p, ctx))
	if err != nil {
		return err
	}

	pr.Result = result
	pr.Message = "Done!"
	pr.Err = nil
	return nil
}

func (p *PostRequest) Parse(r *http.Request) error {
	if r.Method == http.MethodGet {
		// For GET requests, extract from the url query
		if err := p.ParseQueryParameters(r); err != nil {
			return err
		}

	} else {
		// For POST/PUT/DEL requests, extract from the request body instead
		if err := json.NewDecoder(r.Body).Decode(p); err != nil {
			return err
		}
	}

	userID, ok := UserFromContext(r.Context())
	if userID == 0 || !ok {
		return errors.New("userID not found")
	}

	// Set AuthorID if filtering by user's own posts
	if p.MyPosts {
		p.AuthorID = userID
	}

	// Parse if present in request uri (for dynamic routes)
	// Should be executed AFTER decoding request body to overwrite a field of similar name
	if postID := r.PathValue("postID"); postID != "" {
		num, err := strconv.ParseInt(postID, 10, 0)
		if err != nil {
			return err
		}
		p.PostID = int(num)
	}

	return nil
}

func (p *PostRequest) ParseQueryParameters(r *http.Request) error {

	if start := r.URL.Query().Get("start"); start != "" {
		date, err := time.Parse(time.RFC3339, start)
		if err != nil {
			return err
		}
		p.Start = date
	}

	if end := r.URL.Query().Get("end"); end != "" {
		date, err := time.Parse(time.RFC3339, end)
		if err != nil {
			return err
		}
		p.End = date
	}

	if categoryID := r.URL.Query().Get("categoryId"); categoryID != "" {
		num, err := strconv.ParseInt(categoryID, 10, 0)
		if err != nil {
			return err
		}
		p.CategoryID = int(num)
	}

	if myPosts := r.URL.Query().Get("myPosts"); myPosts != "" {
		bl, err := strconv.ParseBool(myPosts)
		if err != nil {
			return err
		}
		p.MyPosts = bl
	}

	if published := r.URL.Query().Get("published"); published != "" {
		bl, err := strconv.ParseBool(published)
		if err != nil {
			return err
		}
		p.Published = &bl
	}
	return nil
}

func scanPost(p *pgxpool.Pool, ctx context.Context) (fn pgx.RowToFunc[*Post]) {
	return func(row pgx.CollectableRow) (*Post, error) {
		var (
			authorID     int
			author       *Author
			reactions    []*Reaction
			commentCount int
			err          error
			x            = &Post{}
		)

		if err := row.Scan(
			&x.Id,
			&x.Title,
			&x.Message,
			&x.CreatedAt,
			&x.UpdatedAt,
			&x.Published,
			&authorID,
			&x.CategoryID,
			&x.IsDeleted,
		); err != nil {
			return nil, err
		}

		// Query all reactions of a post
		if reactions, err = GetReacts(p, ctx, x.Id); err != nil {
			return nil, err
		}

		// Query the author details of a post
		if author, err = FetchAuthor(p, ctx, authorID); err != nil {
			return nil, err
		}

		// Query the total number of comments
		if commentCount, err = GetCommentCount(p, ctx, x.Id); err != nil {
			return nil, err
		}

		x.Reactions = reactions
		x.Count.Comments = commentCount
		// Store the total number of reactions
		x.Count.Reactions = len(reactions)
		x.Author.LastName = author.LastName
		x.Author.FirstName = author.FirstName

		return x, nil
	}
}
