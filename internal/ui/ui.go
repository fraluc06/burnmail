// Package ui owns burnmail's interactive frontends: the Bubble Tea inbox
// and the classic promptui view. It is a leaf package — it reads through
// internal/api and internal/storage, no domain package may import it, and
// cmd/ wires these entry points to cobra.
package ui

import (
	tea "charm.land/bubbletea/v2"

	"burnmail/internal/api"
	"burnmail/internal/storage"
)

// RunInbox starts the Bubble Tea inbox for the stored account and blocks
// until the program quits.
func RunInbox(accountData *storage.AccountData) error {
	client := api.GetClient()
	client.SetToken(accountData.Token)

	p := tea.NewProgram(initialModel(accountData, client))

	_, err := p.Run()
	return err
}
