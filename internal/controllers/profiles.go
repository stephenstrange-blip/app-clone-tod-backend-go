package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ProfileRequest struct {
	UserID    int    `json:"userID,omitzero"`
	FirstName string `json:"firstName,omitzero"`
	LastName  string `json:"lastName,omitzero"`
	Bio       string `json:"bio,omitzero"`
	Title     string `json:"title,omitzero"`
}

type ProfileResponse struct {
	Message string     `json:"message,omitzero"`
	Err     error      `json:"error,omitzero"`
	Result  []*Profile `json:"result,omitzero"`
}

type Profile struct {
	UserID    int           `json:"userID,omitzero"`
	FirstName string        `json:"firstName,omitzero"`
	LastName  string        `json:"lastName,omitzero"`
	Bio       string        `json:"bio,omitzero"`
	Title     string        `json:"title,omitzero"`
	Count     *ProfileCount `json:"count,omitzero"`
}

type ProfileCount struct {
	Followers    int `json:"followers,omitzero"`
	Following    int `json:"following,omitzero"`
	RequestsTo   int `json:"requestsTo,omitzero"`
	RequestsFrom int `json:"requestFrom,omitzero"`
	Comments     int `json:"comments,omitzero"`
	Reaction     int `json:"reaction,omitzero"`
}

func (c *Controller) Profile(pool *pgxpool.Pool) http.HandlerFunc {

	return func(wr http.ResponseWriter, r *http.Request) {
		var response *ProfileResponse
		var err error

		method := r.Method

		if method == "" {
			method = "GET"
		}

		params := &ProfileRequest{}
		if err := params.Parse(r); err != nil {
			fmt.Printf("error (params): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		switch method {
		case "GET":
			response, err = params.GetProfile(pool, r.Context())
		case "PUT":
			response, err = params.PutProfile(pool, r.Context())
		case "POST":
			response, err = params.PostProfile(pool, r.Context())
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

// --------------------- Service Layer -------------------------- //
func (pr *ProfileRequest) PutProfile(p *pgxpool.Pool, ctx context.Context) (*ProfileResponse, error) {
	var (
		argPos    = 1
		response  = &ProfileResponse{}
		setClause = []string{}
		args      = []any{}
	)

	if pr.Bio != "" {
		args = append(args, pr.Bio)
		setClause = append(setClause, fmt.Sprintf(`"bio" = $%d`, argPos))
		argPos++
	}

	if pr.Title != "" {
		args = append(args, pr.Title)
		setClause = append(setClause, fmt.Sprintf(`"title" = $%d`, argPos))
		argPos++
	}

	if pr.FirstName != "" {
		args = append(args, pr.FirstName)
		setClause = append(setClause, fmt.Sprintf(`"firstName" = $%d`, argPos))
		argPos++
	}

	if pr.LastName != "" {
		args = append(args, pr.LastName)
		setClause = append(setClause, fmt.Sprintf(`"lastName" = $%d`, argPos))
		argPos++
	}

	query := fmt.Sprintf(`UPDATE "Profile" SET %s WHERE "userId" = %d`, strings.Join(setClause, ", "), pr.UserID)

	if err := response.UpdateProfile(p, ctx, query, args...); err != nil {
		return nil, err
	}

	return response, nil
}

func (pr *ProfileRequest) PostProfile(p *pgxpool.Pool, ctx context.Context) (*ProfileResponse, error) {
	var (
		argPos   = 4
		cols     = []string{`"userId"`, `"firstName"`, `"lastName"`}
		values   = []string{"$1", "$2", "$3"}
		args     = []any{pr.UserID, pr.FirstName, pr.LastName}
		response = &ProfileResponse{}
	)

	if pr.Bio != "" {
		args = append(args, pr.Bio)
		cols = append(cols, `"bio"`)
		values = append(values, fmt.Sprintf("$%d", argPos))
		argPos++
	}

	if pr.Title != "" {
		args = append(args, pr.Title)
		cols = append(cols, `"title"`)
		values = append(values, fmt.Sprintf("$%d", argPos))
		argPos++
	}

	query := fmt.Sprintf(`INSERT INTO "Profile" (%s) VALUES (%s)`, strings.Join(cols, ", "), strings.Join(values, ", "))

	if err := response.CreateProfile(p, ctx, query, args...); err != nil {
		return nil, err
	}

	return response, nil
}

func (pr *ProfileRequest) GetProfile(p *pgxpool.Pool, ctx context.Context) (*ProfileResponse, error) {
	response := &ProfileResponse{}
	return response, response.FetchProfile(p, ctx, pr.UserID)
}

// --------------------- Repository Layer -------------------------- //
func (pr *ProfileResponse) UpdateProfile(p *pgxpool.Pool, ctx context.Context, query string, args ...any) error {
	if _, err := p.Exec(ctx, query, args...); err != nil {
		return err
	}

	pr.Err = nil
	pr.Result = nil
	pr.Message = "Done!"

	return nil
}

func (pr *ProfileResponse) CreateProfile(p *pgxpool.Pool, ctx context.Context, query string, args ...any) error {
	if _, err := p.Exec(ctx, query, args...); err != nil {
		return err
	}

	pr.Err = nil
	pr.Result = nil
	pr.Message = "Done!"

	return nil
}

func (pr *ProfileResponse) FetchProfile(p *pgxpool.Pool, ctx context.Context, userId int) error {
	x := &Profile{}

	err := p.QueryRow(ctx, `SELECT * FROM "Profile" WHERE  "userId" = $1`, userId).Scan(
		&x.UserID,
		&x.FirstName,
		&x.LastName,
		&x.Title,
		&x.Bio,
	)

	if err != nil {
		return err
	}

	if count, err := getProfileCount(p, ctx, x.UserID); err != nil {
		return err
	} else {
		x.Count = count
	}

	pr.Result = []*Profile{x}
	pr.Message = "Done!"
	pr.Err = nil

	return nil
}

func getProfileCount(p *pgxpool.Pool, ctx context.Context, userId int) (*ProfileCount, error) {
	count := &ProfileCount{}

	// Get number of comments by user
	if err := p.QueryRow(ctx, `SELECT COUNT(*) FROM "Comment" WHERE "authorId" = $1`, userId).Scan(&count.Comments); err != nil {
		return nil, err
	}

	// Get number of user reactions
	if err := p.QueryRow(ctx, `SELECT COUNT(*) FROM "Reactions" WHERE "reactorId" = $1`, userId).Scan(&count.Reaction); err != nil {
		return nil, err
	}

	// Get number of profiles that user follows
	if err := p.QueryRow(ctx, `SELECT COUNT(*) FROM "UserNetwork" WHERE "followingId" = $1`, userId).Scan(&count.Followers); err != nil {
		return nil, err
	}

	// Get number of profiles that follow user
	if err := p.QueryRow(ctx, `SELECT COUNT(*) FROM "UserNetwork" WHERE "followerId" = $1`, userId).Scan(&count.Following); err != nil {
		return nil, err
	}

	// Get number of requests to user
	if err := p.QueryRow(ctx, `SELECT COUNT(*) FROM "FollowRequest" WHERE "targetId" = $1`, userId).Scan(&count.RequestsTo); err != nil {
		return nil, err
	}

	// Get number of requests from user
	if err := p.QueryRow(ctx, `SELECT COUNT(*) FROM "FollowRequest" WHERE "requesterId" = $1`, userId).Scan(&count.RequestsFrom); err != nil {
		return nil, err
	}

	return count, nil
}

func (pr *ProfileRequest) Parse(r *http.Request) error {

	if userId := r.FormValue("userId"); userId != "" {
		if num, err := strconv.ParseInt(userId, 10, 0); err != nil {
			return err
		} else {
			pr.UserID = int(num)
		}
	}

	// Required
	if userId := r.FormValue("userId"); userId != "" {
		if num, err := strconv.ParseInt(userId, 10, 0); err != nil {
			return err
		} else {
			pr.UserID = int(num)
		}
	}

	if firstName := r.FormValue("firstName"); firstName == "" {
		return errors.New("bad Request Body")
	} else {
		pr.FirstName = firstName
	}

	if lastName := r.FormValue("lastName"); lastName == "" {
		return errors.New("bad Request Body")
	} else {
		pr.LastName = lastName
	}

	// Optional
	pr.Title = r.FormValue("title")
	pr.Bio = r.FormValue("bio")

	return nil
}
