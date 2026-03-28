package notifier

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/crypto/verificationhelper"
	mxevent "maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
	"modernc.org/sqlite"

	"tcc-monitor/internal/db"
)

func init() {
	// Register modernc sqlite under the "sqlite3" driver name that mautrix expects.
	for _, name := range sql.Drivers() {
		if name == "sqlite3" {
			return
		}
	}
	sql.Register("sqlite3", &sqlite.Driver{})
}

type Notifier struct {
	client       *mautrix.Client
	cryptoHelper *cryptohelper.CryptoHelper
	verifier     *verificationhelper.VerificationHelper
	cancel       context.CancelFunc
}

// autoVerifyCallbacks implements the verification callbacks to auto-accept
// emoji verification requests.
type autoVerifyCallbacks struct {
	verifier *verificationhelper.VerificationHelper
}

func (a *autoVerifyCallbacks) VerificationRequested(ctx context.Context, txnID id.VerificationTransactionID, from id.UserID, fromDevice id.DeviceID) {
	log.Printf("matrix: verification requested by %s (device %s) txn=%s, auto-accepting", from, fromDevice, txnID)
	go func() {
		if err := a.verifier.AcceptVerification(ctx, txnID); err != nil {
			log.Printf("matrix: failed to accept verification: %v", err)
		} else {
			log.Println("matrix: accept verification sent successfully")
		}
	}()
}

func (a *autoVerifyCallbacks) VerificationCancelled(ctx context.Context, txnID id.VerificationTransactionID, code mxevent.VerificationCancelCode, reason string) {
	log.Printf("matrix: verification cancelled: %s (%s)", reason, code)
}

func (a *autoVerifyCallbacks) VerificationDone(ctx context.Context, txnID id.VerificationTransactionID) {
	log.Println("matrix: verification complete! Device is now verified.")
}

func (a *autoVerifyCallbacks) ShowSAS(ctx context.Context, txnID id.VerificationTransactionID, emojis []rune, emojiDescriptions []string, decimals []int) {
	log.Printf("matrix: SAS emojis: %v (%v) — auto-confirming", string(emojis), emojiDescriptions)
	go func() {
		if err := a.verifier.ConfirmSAS(ctx, txnID); err != nil {
			log.Printf("matrix: failed to confirm SAS: %v", err)
		} else {
			log.Println("matrix: SAS confirmation sent successfully")
		}
	}()
}

func New(ctx context.Context, homeserver, username, password, pickleKey, cryptoDBPath string, database *db.DB) (*Notifier, error) {
	client, err := mautrix.NewClient(homeserver, "", "")
	if err != nil {
		return nil, fmt.Errorf("create matrix client: %w", err)
	}

	// Reuse the device ID from a previous session if available.
	loginReq := &mautrix.ReqLogin{
		Type: mautrix.AuthTypePassword,
		Identifier: mautrix.UserIdentifier{
			Type: mautrix.IdentifierTypeUser,
			User: username,
		},
		Password:         password,
		StoreCredentials: true,
	}
	if savedDeviceID, err := database.GetSetting("matrix_device_id"); err == nil && savedDeviceID != "" {
		loginReq.DeviceID = id.DeviceID(savedDeviceID)
	}

	resp, err := client.Login(ctx, loginReq)
	if err != nil {
		return nil, fmt.Errorf("matrix login: %w", err)
	}
	log.Printf("matrix: logged in as %s (device %s)", client.UserID, resp.DeviceID)

	// Persist the device ID for next restart.
	database.SetSetting("matrix_device_id", string(resp.DeviceID))

	// Set up E2EE with persistent crypto store.
	cryptoHelper, err := cryptohelper.NewCryptoHelper(client, []byte(pickleKey), cryptoDBPath)
	if err != nil {
		return nil, fmt.Errorf("crypto helper: %w", err)
	}

	if err := cryptoHelper.Init(ctx); err != nil {
		return nil, fmt.Errorf("crypto init: %w", err)
	}
	client.Crypto = cryptoHelper

	// Set up auto-accept emoji verification.
	callbacks := &autoVerifyCallbacks{}
	verifier := verificationhelper.NewVerificationHelper(client, cryptoHelper.Machine(), nil, callbacks, false)
	callbacks.verifier = verifier
	if err := verifier.Init(ctx); err != nil {
		log.Printf("matrix: verification helper init failed: %v", err)
	}

	// Start sync loop in background.
	syncCtx, cancel := context.WithCancel(ctx)
	go func() {
		log.Println("matrix: starting sync loop")
		if err := client.SyncWithContext(syncCtx); err != nil && syncCtx.Err() == nil {
			log.Printf("matrix: sync error: %v", err)
		}
	}()

	return &Notifier{
		client:       client,
		cryptoHelper: cryptoHelper,
		verifier:     verifier,
		cancel:       cancel,
	}, nil
}

func (n *Notifier) SendAlert(ctx context.Context, roomID string, plain string, html string) error {
	_, err := n.client.SendMessageEvent(ctx, id.RoomID(roomID), mxevent.EventMessage, &mxevent.MessageEventContent{
		MsgType:       mxevent.MsgText,
		Body:          plain,
		Format:        mxevent.FormatHTML,
		FormattedBody: html,
	})
	if err != nil {
		return fmt.Errorf("send matrix alert: %w", err)
	}
	return nil
}

func (n *Notifier) Stop() {
	log.Println("matrix: shutting down")
	n.cancel()
	if n.cryptoHelper != nil {
		n.cryptoHelper.Close()
	}
}
