package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProfileNetworkRequest struct {
	MyFollowers bool `json:"myFollowers,omitzero"`
	UserID      int  `json:"userID,omitzero"`
	FollowerId  int  `json:"followerId,omitzero"`
}

type ProfileNetworkResponse struct {
	Message string            `json:"message,omitzero"`
	Err     error             `json:"err,omitzero"`
	Result  []*ProfileNetwork `json:"result"`
}

type ProfileNetwork struct {
	Name       Author    `json:"name,omitzero"`
	UserID     int       `json:"userId,omitzero"`
	AssignedAt time.Time `json:"assignedAt,omitzero"`
}

func (c *Controller) Network(pool *pgxpool.Pool) http.HandlerFunc {
	var (
		response *ProfileNetworkResponse
		err      error
	)
	return func(wr http.ResponseWriter, r *http.Request) {
		method := r.Method

		params := &ProfileNetworkRequest{}
		if err := params.Parse(r); err != nil {
			fmt.Printf("error (internal): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		switch method {
		case "GET":
			response, err = params.GetNetwork(pool, r.Context())
		case "POST":
			response, err = params.PostNetwork(pool, r.Context())
		case "DEL":
			response, err = params.DelNetwork(pool, r.Context())
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

func (pn *ProfileNetworkRequest) GetNetwork(p *pgxpool.Pool, ctx context.Context) (*ProfileNetworkResponse, error) {
	var (
		query    string
		response = &ProfileNetworkResponse{}
	)

	if pn.FollowerId == 0 || pn.UserID == 0 {
		return nil, errors.New("bad request body")
	}

	if pn.MyFollowers {
		// fetch all that follow userId OR
		query = `SELECT "followerId", "assignedAt" FROM "UserNetwork" WHERE "followingId" = $1`
	} else {
		// fetch all that userId follows
		query = `SELECT "followingId", "assignedAt" FROM "UserNetwork" WHERE "followerId" = $1`
	}

	if err := response.FetchNetwork(p, ctx, query, pn.UserID); err != nil {
		return nil, err
	}

	return response, nil
}

func (pn *ProfileNetworkRequest) DelNetwork(p *pgxpool.Pool, ctx context.Context) (*ProfileNetworkResponse, error) {
	if pn.FollowerId == 0 || pn.UserID == 0 {
		return nil, errors.New("bad request body")
	}

	response := &ProfileNetworkResponse{}

	if err := response.RemoveNetwork(p, ctx, pn.FollowerId, pn.UserID); err != nil {
		return nil, err
	}

	return response, nil
}

func (pn *ProfileNetworkRequest) PostNetwork(p *pgxpool.Pool, ctx context.Context) (*ProfileNetworkResponse, error) {
	if pn.FollowerId == 0 || pn.UserID == 0 {
		return nil, errors.New("bad request body")
	}

	response := &ProfileNetworkResponse{}

	if err := response.CreateNetwork(p, ctx, pn.FollowerId, pn.UserID); err != nil {
		return nil, err
	}

	return response, nil
}

// --------------------- Repository Layer -------------------------- //

func (pn *ProfileNetworkResponse) RemoveNetwork(p *pgxpool.Pool, ctx context.Context, followerId, userID int) error {
	result, err := p.Exec(ctx, `DELETE FROM ONLY "UserNetwork" WHERE "followerId" = $1 AND "followingId" = $2`, followerId, userID)
	if err != nil {
		return err
	}

	if result.RowsAffected() != 1 {
		return errors.New("operation affected more or less than 1 row. Contact tech support immediately")
	}

	pn.Err = nil
	pn.Message = "Done!"
	pn.Result = nil

	return nil
}

func (pn *ProfileNetworkResponse) CreateNetwork(p *pgxpool.Pool, ctx context.Context, followerID, userID int) error {
	x := &ProfileNetwork{
		UserID: followerID,
	}

	err := p.QueryRow(ctx, `INSERT INTO "UserNetwork" ("followerId", "followingId") VALUES ($1, $2) RETURNING "assignedAt"`, followerID, userID).Scan(&x.AssignedAt)
	if err != nil {
		return err
	}

	pn.Err = nil
	pn.Message = "Done!"
	pn.Result = []*ProfileNetwork{x}

	return nil
}

func (pn *ProfileNetworkResponse) FetchNetwork(p *pgxpool.Pool, ctx context.Context, query string, userID int) error {
	rows, _ := p.Query(ctx, query, userID)

	result, err := pgx.CollectRows(rows, scanNetwork(p, ctx))
	if err != nil {
		return err
	}

	pn.Err = nil
	pn.Message = "Done!"
	pn.Result = result

	return nil
}

func scanNetwork(p *pgxpool.Pool, ctx context.Context) (fn pgx.RowToFunc[*ProfileNetwork]) {
	return func(row pgx.CollectableRow) (*ProfileNetwork, error) {
		author := &Author{}
		x := &ProfileNetwork{}
		var err error

		if err = row.Scan(&x.UserID, &x.AssignedAt); err != nil {
			return nil, err
		}

		// Get details (AKA Author details)
		if author, err = FetchAuthor(p, ctx, x.UserID); err != nil {
			return nil, err
		}

		x.Name.FirstName = author.FirstName
		x.Name.LastName = author.LastName

		return x, nil
	}
}

func (pr *ProfileNetworkRequest) Parse(r *http.Request) error {

	userID, ok := UserFromContext(r.Context())
	if !ok {
		return errors.New("userID not found")
	}

	pr.UserID = userID

	if myFollowers := r.FormValue("myFollowers"); myFollowers != "" {
		if bl, err := strconv.ParseBool(myFollowers); err != nil {
			return err
		} else {
			pr.MyFollowers = bl
		}
	}

	if followerID := r.FormValue("followerId"); followerID != "" {
		if num, err := strconv.ParseInt(followerID, 10, 0); err != nil {
			return err
		} else {
			pr.FollowerId = int(num)
		}
	}

	return nil
}
