package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FollowNetworkRequest struct {
	TargetID     int  `json:"targetId,omitzero"`
	RequesterID  int  `json:"requesterId,omitzero"`
	FromRequests bool `json:"fromRequests,omitempty"`
	UserID       int  `json:"userID,omitzero"`
}

type FollowNetworkResponse struct {
	Message string           `json:"message,omitzero"`
	Err     error            `json:"error,omitzero"`
	Result  []*FollowNetwork `json:"result,omitzero"`
}

type FollowNetwork struct {
	TargetID      int       `json:"targetId,omitzero"`
	TargetName    Author    `json:"targetName,omitzero"`
	RequesterName Author    `json:"requesterName,omitzero"`
	RequesterID   int       `json:"requesterId,omitzero"`
	CreatedAt     time.Time `json:"createdAt,omitzero"`
}

func (c *Controller) Request(pool *pgxpool.Pool) http.HandlerFunc {
	return func(wr http.ResponseWriter, r *http.Request) {
		var response *FollowNetworkResponse
		var err error

		method := r.Method
		if method == "" {
			method = "GET"
		}

		params := FollowNetworkRequest{}
		if err := params.Parse(r); err != nil {
			fmt.Printf("error (params): %s\n", err.Error())
			wr.WriteHeader(http.StatusBadRequest)
		}

		switch method {
		case http.MethodPost:
			response, err = params.PostFollowNetwork(pool, r.Context())
		case http.MethodGet:
			response, err = params.GetFollowNetwork(pool, r.Context())
		case http.MethodDelete:
			response, err = params.DelFollowNetwork(pool, r.Context())
		default:
			wr.WriteHeader(http.StatusMethodNotAllowed)
		}

		if err != nil {
			fmt.Printf("error (internal): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		if p, err := json.Marshal(response); err != nil {
			http.Error(wr, err.Error(), http.StatusInternalServerError)
			return
		} else {
			wr.Write(p)
		}

	}
}

// --------------------- Service Layer -------------------------- //

func (p *FollowNetworkRequest) Parse(r *http.Request) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	// Extracts TargetID and/or RequesterID (optional)
	if err := json.Unmarshal(body, p); err != nil {
		return err
	}

	fromRequests := r.URL.Query().Get("fromRequests")

	// Used when fetching followRequests (optional)
	if fromRequests != "" {
		bl, err := strconv.ParseBool(fromRequests)
		if err != nil {
			return err
		}

		p.FromRequests = bl
	}

	// Required
	userID, ok := UserFromContext(r.Context())
	if !ok || userID == 0 {
		return errors.New("userID not found")
	}
	// Dont forget to set userID!
	p.UserID = userID

	return nil
}

func (p *FollowNetworkRequest) PostFollowNetwork(pool *pgxpool.Pool, ctx context.Context) (*FollowNetworkResponse, error) {
	response := &FollowNetworkResponse{}

	if p.TargetID == 0 {
		return nil, errors.New("bad request body")
	}

	// Logged-in user makes a request to another user
	if err := response.CreateFollowNetwork(pool, ctx, p.TargetID, p.UserID); err != nil {
		return nil, err
	}

	return response, nil
}

func (p *FollowNetworkRequest) GetFollowNetwork(pool *pgxpool.Pool, ctx context.Context) (*FollowNetworkResponse, error) {
	response := &FollowNetworkResponse{}

	if err := response.FetchFollowNetwork(pool, ctx, p.UserID, p.FromRequests); err != nil {
		return nil, err
	}

	return response, nil
}

func (p *FollowNetworkRequest) DelFollowNetwork(pool *pgxpool.Pool, ctx context.Context) (*FollowNetworkResponse, error) {
	var (
		response = &FollowNetworkResponse{}
		err      error
	)

	if p.TargetID == 0 {
		return nil, errors.New("bad request body")
	}

	if p.RequesterID == 0 {
		return nil, errors.New("bad request body")
	}

	if err = response.RemoveFollowNetwork(pool, ctx, p.TargetID, p.RequesterID); err != nil {
		return nil, err
	}

	return response, nil
}

// --------------------- Repository Layer -------------------------- //

func (r *FollowNetworkResponse) CreateFollowNetwork(p *pgxpool.Pool, ctx context.Context, targetID, requesterID int) error {
	x := &FollowNetwork{
		TargetID:    targetID,
		RequesterID: requesterID,
	}

	err := p.QueryRow(ctx, `INSERT INTO "FollowRequest" ("requesterId", "targetId") VALUES ($1, $2) RETURNING "createdAt"`, requesterID, targetID).Scan(&x.CreatedAt)
	if err != nil {
		return err
	}

	r.Result = []*FollowNetwork{x}
	r.Err = nil
	r.Message = "Done!"
	return nil
}

func (r *FollowNetworkResponse) FetchFollowNetwork(p *pgxpool.Pool, ctx context.Context, id int, fromRequests bool) error {
	var query string

	if fromRequests {
		// Query all requests made by client
		query = `SELECT "createdAt", "targetId" FROM "FollowRequest" WHERE "requesterId" = $1`
	} else {
		// Query all requests to client
		query = `SELECT "createdAt", "requesterId" FROM "FollowRequest" WHERE "targetId" = $1`
	}

	rows, _ := p.Query(ctx, query, id)
	result, err := pgx.CollectRows(rows, scanFollowNetwork(p, ctx, fromRequests))
	if err != nil {
		return err
	}

	r.Result = result
	r.Message = "Done!"
	r.Err = nil
	return nil
}

func (r *FollowNetworkResponse) RemoveFollowNetwork(p *pgxpool.Pool, ctx context.Context, targetID, requesterID int) error {
	result, err := p.Exec(ctx, `DELETE FROM ONLY "FollowRequest" WHERE "targetId" = $1 AND "requesterId" = $2`, targetID, requesterID)

	if err != nil {
		return err
	}

	if result.RowsAffected() != 1 {
		return errors.New("operation affected more or less than 1 row. contact tech support immediately")
	}

	r.Message = "Done!"
	r.Err = nil
	return nil
}

func scanFollowNetwork(p *pgxpool.Pool, ctx context.Context, fromRequests bool) pgx.RowToFunc[*FollowNetwork] {
	return func(row pgx.CollectableRow) (*FollowNetwork, error) {
		var (
			x      = &FollowNetwork{}
			author *Author
			err    error
		)

		if fromRequests {
			// Populate target profile names that client is requesting to follow
			if err = row.Scan(&x.CreatedAt, &x.TargetID); err != nil {
				return nil, err
			}

			if author, err = FetchAuthor(p, ctx, x.TargetID); err != nil {
				return nil, err
			}

			x.TargetName.FirstName = author.FirstName
			x.TargetName.LastName = author.LastName

		} else {
			// Populate requester profile names that requests to follow client
			if err = row.Scan(&x.CreatedAt, &x.RequesterID); err != nil {
				return nil, err
			}

			if author, err = FetchAuthor(p, ctx, x.RequesterID); err != nil {
				return nil, err
			}

			x.RequesterName.FirstName = author.FirstName
			x.RequesterName.LastName = author.LastName
		}

		return x, nil
	}
}
