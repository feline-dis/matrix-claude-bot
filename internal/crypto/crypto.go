package crypto

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"go.mau.fi/util/dbutil"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"

	_ "modernc.org/sqlite"

	"github.com/feline-dis/matrix-claude-bot/internal/config"
)

// Setup initializes E2EE support using mautrix cryptohelper. Returns the
// CryptoHelper which the caller should Close on shutdown.
func Setup(ctx context.Context, client *mautrix.Client, cfg config.Config) (*cryptohelper.CryptoHelper, error) {
	db, err := openDatabase(cfg.CryptoDatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open crypto database: %w", err)
	}

	helper, err := cryptohelper.NewCryptoHelper(client, []byte(cfg.PickleKey), db)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create crypto helper: %w", err)
	}

	if err := helper.Init(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize crypto helper: %w", err)
	}

	client.Crypto = helper
	log.Println("E2EE support enabled")
	return helper, nil
}

func openDatabase(path string) (*dbutil.Database, error) {
	dsn := fmt.Sprintf("file:%s?_txlock=immediate&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)", path)
	rawDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	return dbutil.NewWithDB(rawDB, "sqlite")
}
