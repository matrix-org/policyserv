package homeserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/matrix-org/policyserv/pubsub"
)

func (h *Homeserver) waitForEdus(ch <-chan string) {
	// We should already be called from a gofunc, so we can jump right in

	for val := range ch {
		if val == pubsub.ClosingValue {
			return // stop getting values
		}

		log.Printf("Received notification of new EDU for %s - sending transaction", val)
		h.SendNextTransactionTo(context.Background(), val)
	}
}

func (h *Homeserver) SendNextTransactionTo(ctx context.Context, destination string) {
	// Note: we don't return errors to callers because we "handle" them internally here. We also expect
	// this function to be called multiple times concurrently, so we don't really want to make callers
	// responsible for logging errors (which would result in duplicate logs).

	// We can accumulate open database connections if we're not careful here, so limit our calls to
	// a minute and put a singleflight over each destination to prevent concurrent calls.
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	// ⚠️ The function passed to singleflight.Do only runs the "first" time, meaning the context we have
	// might not be the same context which produced an (error) response from the singleflight. This isn't
	// really a problem here though, but is worth noting if we pull more variables from outside the scope
	// in the future.
	defer h.sendTxnSingleflight.Forget(destination)
	txnId, err, _ := h.sendTxnSingleflight.Do(destination, func() (interface{}, error) {
		mxTxn, sqlTxn, err := h.storage.BeginMatrixTransaction(ctx, destination)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil // nothing to send, so we're successful
			}
			return nil, err
		}

		defer sqlTxn.Rollback() // if something goes wrong, we want to rollback. Does nothing if called after Commit()

		log.Printf("[%s] Sending %d EDUs to %s", mxTxn.TransactionId, len(mxTxn.Edus), destination)
		_, err = h.client.SendTransaction(ctx, gomatrixserverlib.Transaction{
			TransactionID:  gomatrixserverlib.TransactionID(mxTxn.TransactionId),
			Origin:         h.ServerName,
			Destination:    spec.ServerName(destination),
			OriginServerTS: spec.Timestamp(time.Now().UnixMilli()),
			PDUs:           make([]json.RawMessage, 0), // we don't have any PDUs to send
			EDUs:           mxTxn.Edus,
		})
		if err != nil {
			return mxTxn.TransactionId, err
		}

		// There's nothing valuable in the response for us, so just commit our transaction and move on
		return mxTxn.TransactionId, sqlTxn.Commit()
	})
	if err != nil {
		log.Printf("[%v] Error sending transaction to %s: %s", txnId, destination, err)
	} else {
		log.Printf("[%v] Successfully sent transaction to %s", txnId, destination)
	}
}
