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

type Reaction struct {
	Id        int       `json:"id,omitzero"`
	ReactID   int       `json:"reactID,omitzero"`
	PostID    int       `json:"postID,omitzero"`
	CreatedAt time.Time `json:"createdAt,omitzero"`
	UpdatedAt time.Time `json:"updatedAt,omitzero"`
	ReactorID int       `json:"reactorID,omitzero"`
}

type ReactionRequest struct {
	Id_react  int `json:"id_react,omitzero"` // the row id
	PostID    int `json:"postID,omitzero"`
	ReactorID int `json:"reactorID,omitzero"`
	ReactID   int `json:"reactID,omitzero"` // lists what type of react from "React" table
}

type ReactionResponse struct {
	Message string `json:"message,omitzero"`
}

func (x *ReactionRequest) Parse(r *http.Request) error {

	switch r.Method {
	// only one parameter is required from DEL requests
	case http.MethodDelete:
		id_react := r.URL.Query().Get("id_react")
		if id_react == "" {
			return errors.New("bad request body")
		}

		num, err := strconv.ParseInt(id_react, 10, 0)
		if err != nil {
			return err
		}
		x.Id_react = int(num)

	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(body, x); err != nil {
			return err
		}

		userID, ok := UserFromContext(r.Context())
		if !ok || userID == 0 {
			return errors.New("userID not found")
		}

		// logged-in users are always the reactor when creating reacts
		x.ReactorID = userID
	}

	return nil
}

func (c *Controller) Reaction(pool *pgxpool.Pool) http.HandlerFunc {

	return func(wr http.ResponseWriter, r *http.Request) {
		var response *ReactionResponse
		var err error

		method := r.Method

		params := &ReactionRequest{}
		if err := params.Parse(r); err != nil {
			fmt.Printf("error (params): %s", err.Error())
			wr.WriteHeader(http.StatusBadRequest)
			return
		}

		switch method {
		case http.MethodPost:
			err = params.PostReact(pool, r.Context())
		case http.MethodDelete:
			err = params.DelReact(pool, r.Context())
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

func (p *ReactionRequest) PostReact(pool *pgxpool.Pool, ctx context.Context) error {
	response := &ReactionResponse{}
	if p.ReactorID == 0 || p.PostID == 0 {
		return errors.New("bad request body")
	}

	if err := response.CreateReact(pool, ctx, p.ReactID, p.ReactorID, p.PostID); err != nil {
		return err
	}

	return nil
}

func (p *ReactionRequest) DelReact(pool *pgxpool.Pool, ctx context.Context) error {
	response := &ReactionResponse{}

	if p.Id_react == 0 {
		return errors.New("bad request body")
	}

	if err := response.RemoveReact(pool, ctx, p.Id_react); err != nil {
		return err
	}

	return nil
}

// --------------------- Repository Layer -------------------------- //

func (p *ReactionResponse) CreateReact(pool *pgxpool.Pool, ctx context.Context, reactID, reactorID, postID int) error {
	timeNow := time.Now()
	x := &Reaction{}

	err := pool.QueryRow(
		ctx,
		`INSERT INTO "Reactions" ("reactId", "postId","createdAt","updatedAt","reactorId") VALUES ($1, $2, $3, $4, $5) RETURNING *`,
		reactID, postID, timeNow, timeNow, reactorID,
	).Scan(&x.Id, &x.ReactID, &x.PostID, &x.CreatedAt, &x.UpdatedAt, &x.ReactorID)

	if err != nil {
		return err
	}

	p.Message = "Done!"

	return nil
}

func (p *ReactionResponse) RemoveReact(pool *pgxpool.Pool, ctx context.Context, id_react int) error {
	x := &Reaction{}

	err := pool.QueryRow(ctx, `DELETE FROM ONLY "Reactions" WHERE id = $1 RETURNING *`, id_react).Scan(&x.Id, &x.ReactID, &x.PostID, &x.CreatedAt, &x.UpdatedAt, &x.ReactorID)
	if err != nil {
		return err
	}

	p.Message = "Done!"
	return nil
}

// Used by FetchPost(s) handlers in posts.go
func GetReacts(p *pgxpool.Pool, ctx context.Context, postID int) ([]*Reaction, error) {
	rows, _ := p.Query(ctx, `SELECT "reactorId", "reactId", id FROM "Reactions" WHERE "postId" = $1`, postID)

	result, err := pgx.CollectRows(rows, scanReaction)

	if err != nil {
		return nil, err
	}

	return result, nil
}

func scanReaction(row pgx.CollectableRow) (*Reaction, error) {
	x := &Reaction{}

	if err := row.Scan(&x.ReactorID, &x.ReactID, &x.Id); err != nil {
		return nil, err
	}

	return x, nil
}
