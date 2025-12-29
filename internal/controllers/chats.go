package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	customUtil "github.com/app-clone-tod-utils"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Message struct {
	content   string
	authorID  int
	roomId    int
	updatedAt time.Time
	id        int
}

func (c *Controller) Chat(pool *pgxpool.Pool) http.HandlerFunc {
	return func(wr http.ResponseWriter, r *http.Request) {
		chatID := r.PathValue("chatID")

		id, err := strconv.ParseInt(chatID, 10, 0)
		if err != nil {
			fmt.Printf("error (internal): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Automatically closes pool inside
		err = getMessage(pool, r.Context(), int(id))
		if err != nil {
			fmt.Printf("error (internal): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		message := fmt.Sprintf("Fetching chat messages from chat %d", id)

		if p, err := json.Marshal(&ReactionResponse{Message: message}); err != nil {
			fmt.Printf("error (internal): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		} else {
			wr.Write(p)
		}

	}
}

func getMessage(p *pgxpool.Pool, ctx context.Context, id int) error {
	defer p.Close()

	rows, _ := p.Query(ctx, `SELECT * FROM "Messages" WHERE "roomId" = $1`, id)

	result, err := pgx.CollectRows(rows, scanMessage)
	if err != nil {
		return err
	}

	logger := customUtil.NewCustomLogger()

	for _, row := range result {
		logger.Info("Get",
			slog.Group("Chat",
				slog.Int("id", row.id),
				slog.String("content", row.content),
				slog.Int("authorId", row.authorID),
				slog.Int("roomId", row.roomId),
				slog.Time("updatedAt", row.updatedAt),
			),
		)
	}

	return nil
}

func scanMessage(row pgx.CollectableRow) (*Message, error) {
	x := &Message{}

	// Consider order of columns as it appears on db
	err := row.Scan(&x.id, nil, &x.content, nil, &x.updatedAt, &x.authorID, &x.roomId)
	if err != nil {
		return x, err
	}

	return x, nil
}
