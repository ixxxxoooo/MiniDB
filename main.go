// MiniDB is an AI-assisted desktop database client written with MyGo's
// native UI. Dependencies point one way: ui → app → domain packages.
package main

import (
	"fmt"
	"os"

	"github.com/egoist/mygo"

	"minidb/internal/app"
	"minidb/internal/appdata"
	"minidb/internal/keychain"
	"minidb/internal/logger"
	"minidb/internal/storage"
	"minidb/internal/ui"
	_ "minidb/locales"
)

func main() {
	if err := logger.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "log init failed: %v\n", err)
	}
	defer logger.Close()

	// A second launch hands over to the running instance.
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}

	_ = appdata.MigrateLegacyFiles()
	// MINIDB_NO_KEYCHAIN keeps the encryption key in a file of the data
	// directory instead of the login keychain, for tests and headless runs.
	if os.Getenv("MINIDB_NO_KEYCHAIN") == "" {
		// 开发版（mygo dev）每次重编译签名都会变，钥匙串会反复弹授权框并卡住主线程，
		// 所以开发版优先用磁盘上已有的密钥文件；正式版始终走钥匙串。
		storage.SetKeyProvider(keychain.Provider{Backend: keychain.System{}, PreferFile: mygo.IsDev()})
	}
	store, err := storage.NewStore()
	if err != nil {
		logger.Error("open store: %v", err)
		fmt.Fprintf(os.Stderr, "cannot open the local database: %v\n", err)
		os.Exit(1)
	}
	if ierr := store.ImportError(); ierr != nil {
		logger.Warn("legacy data import failed: %v", ierr)
	}

	var shell *ui.Shell
	a := app.New(store, ui.Host(func() *mygo.Window { return shell.Window() }))
	shell = ui.NewShell(a, ui.Host(func() *mygo.Window { return shell.Window() }), false)
	a.Start()
	defer a.Shutdown()
	defer shell.Close()

	mygo.App.OnSecondInstance(func([]string, string) { shell.Focus() })
	mygo.App.WhenReady(shell.Open)

	if err := mygo.App.Run(); err != nil {
		logger.Error("run failed: %v", err)
		os.Exit(1)
	}
}
