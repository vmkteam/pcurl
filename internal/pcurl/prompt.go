package pcurl

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/vmkteam/pcurl/internal/curlparse"

	huh "charm.land/huh/v2"
)

// StorageChoice is where to store a secret value.
type StorageChoice int

const (
	StoreKeychain StorageChoice = iota
	StoreConfig
	StoreSkip
)

var (
	storageOptionsAll = []huh.Option[StorageChoice]{
		huh.NewOption("Keychain (recommended)", StoreKeychain),
		huh.NewOption("Config (plaintext)", StoreConfig),
		huh.NewOption("Skip", StoreSkip),
	}
	storageOptionsNoSkip = storageOptionsAll[:2]
)

// Prompter handles all interactive prompts.
// When Interactive is true, uses huh TUI (arrow keys, space to toggle).
// Otherwise falls back to plain text input (for tests, pipes, non-TTY).
type Prompter struct {
	scanner     *bufio.Scanner
	Out         io.Writer
	Interactive bool
}

// NewPrompter creates a Prompter with shared scanner.
func NewPrompter(in io.Reader, out io.Writer, interactive bool) *Prompter {
	return &Prompter{
		scanner:     bufio.NewScanner(in),
		Out:         out,
		Interactive: interactive,
	}
}

func (p *Prompter) readLine() (string, error) {
	if p.scanner.Scan() {
		return p.scanner.Text(), nil
	}
	if err := p.scanner.Err(); err != nil {
		return "", err
	}
	return "", io.EOF
}

// Storage asks where to store a secret header.
func (p *Prompter) Storage(headerName, maskedValue string) (StorageChoice, error) {
	if p.Interactive {
		return p.storageHuh(headerName, maskedValue)
	}
	return p.storageText(headerName, maskedValue)
}

func (p *Prompter) storageHuh(headerName, maskedValue string) (StorageChoice, error) {
	var choice StorageChoice
	err := huh.NewSelect[StorageChoice]().
		Title(fmt.Sprintf("Secret: %s: %s", headerName, maskedValue)).
		Options(storageOptionsAll...).
		Value(&choice).
		Run()
	return choice, err
}

func (p *Prompter) storageText(headerName, maskedValue string) (StorageChoice, error) {
	fmt.Fprintf(p.Out, "  Secret: %s: %s\n", headerName, maskedValue)
	fmt.Fprintf(p.Out, "  Store in: [K]eychain  [C]onfig (plaintext)  [S]kip?  [k]: ")

	answer, err := p.readLine()
	if err != nil {
		return StoreKeychain, err
	}

	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "c":
		return StoreConfig, nil
	case "s":
		return StoreSkip, nil
	default:
		return StoreKeychain, nil
	}
}

// CookieStorage asks where to store selected cookies.
func (p *Prompter) CookieStorage() (StorageChoice, error) {
	if p.Interactive {
		return p.cookieStorageHuh()
	}
	return p.cookieStorageText()
}

func (p *Prompter) cookieStorageHuh() (StorageChoice, error) {
	var choice StorageChoice
	err := huh.NewSelect[StorageChoice]().
		Title("Store selected cookies in:").
		Options(storageOptionsNoSkip...).
		Value(&choice).
		Run()
	return choice, err
}

func (p *Prompter) cookieStorageText() (StorageChoice, error) {
	fmt.Fprintf(p.Out, "  Store selected cookies in: [K]eychain  [C]onfig (plaintext)?  [k]: ")

	answer, err := p.readLine()
	if err != nil {
		return StoreKeychain, err
	}

	if strings.ToLower(strings.TrimSpace(answer)) == "c" {
		return StoreConfig, nil
	}
	return StoreKeychain, nil
}

// ConfirmUpdate asks "Profile X already exists. Update? [Y/n]".
func (p *Prompter) ConfirmUpdate(profileName string) (bool, error) {
	if p.Interactive {
		return p.confirmUpdateHuh(profileName)
	}
	return p.confirmUpdateText(profileName)
}

func (p *Prompter) confirmUpdateHuh(profileName string) (bool, error) {
	var confirm bool
	err := huh.NewConfirm().
		Title(fmt.Sprintf("Profile %q already exists. Update?", profileName)).
		Affirmative("Yes").
		Negative("No").
		Value(&confirm).
		Run()
	return confirm, err
}

func (p *Prompter) confirmUpdateText(profileName string) (bool, error) {
	fmt.Fprintf(p.Out, "Profile %q already exists. Update? [Y/n]: ", profileName)

	answer, err := p.readLine()
	if err != nil {
		return true, err
	}

	return strings.ToLower(strings.TrimSpace(answer)) != "n", nil
}

// PickHeaders shows non-secret headers and lets the user toggle selection.
func (p *Prompter) PickHeaders(headers []curlparse.Header) error {
	if p.Interactive {
		return p.pickHeadersHuh(headers)
	}
	return p.pickHeadersText(headers)
}

func (p *Prompter) pickHeadersHuh(headers []curlparse.Header) error {
	var options []huh.Option[int]
	var selected []int

	for i, h := range headers {
		if h.Secret {
			continue
		}
		options = append(options, huh.NewOption(fmt.Sprintf("%s: %s", h.Name, h.Value), i))
		if h.Selected {
			selected = append(selected, i)
		}
	}

	if len(options) == 0 {
		return nil
	}

	err := huh.NewMultiSelect[int]().
		Title(fmt.Sprintf("Select headers to save (%d found)", len(options))).
		Options(options...).
		Value(&selected).
		Height(15).
		Run()
	if err != nil {
		return err
	}

	applySelection(len(headers), selected, func(i int) bool { return headers[i].Secret }, func(i int, v bool) { headers[i].Selected = v })
	return nil
}

func (p *Prompter) pickHeadersText(headers []curlparse.Header) error {
	// count non-secret headers
	var count int
	for _, h := range headers {
		if !h.Secret {
			count++
		}
	}
	if count == 0 {
		return nil
	}

	fmt.Fprintf(p.Out, "\nHeaders found (%d). Select headers to save:\n", count)
	idxMap := make([]int, 0, count) // display index → headers index
	for i, h := range headers {
		if h.Secret {
			continue
		}
		marker := " "
		if h.Selected {
			marker = "x"
		}
		idxMap = append(idxMap, i)
		fmt.Fprintf(p.Out, "  %d. [%s] %s: %s\n", len(idxMap), marker, h.Name, h.Value)
	}

	fmt.Fprintf(p.Out, "\nToggle by number (e.g. \"3 5\"), [a]ll, [n]one, or enter to confirm: ")

	answer, err := p.readLine()
	if err != nil {
		return err
	}
	answer = strings.TrimSpace(answer)

	switch strings.ToLower(answer) {
	case "a":
		for _, idx := range idxMap {
			headers[idx].Selected = true
		}
	case "n":
		for _, idx := range idxMap {
			headers[idx].Selected = false
		}
	case "":
		// keep
	default:
		for _, tok := range strings.Fields(answer) {
			var num int
			if _, err := fmt.Sscanf(tok, "%d", &num); err == nil && num >= 1 && num <= len(idxMap) {
				idx := idxMap[num-1]
				headers[idx].Selected = !headers[idx].Selected
			}
		}
	}

	selected := 0
	for _, idx := range idxMap {
		if headers[idx].Selected {
			selected++
		}
	}
	fmt.Fprintf(p.Out, "Selected %d of %d headers\n", selected, count)
	return nil
}

// PickCookies shows cookies and lets the user toggle selection.
func (p *Prompter) PickCookies(cookies []curlparse.Cookie) error {
	if p.Interactive {
		return p.pickCookiesHuh(cookies)
	}
	return p.pickCookiesText(cookies)
}

func (p *Prompter) pickCookiesHuh(cookies []curlparse.Cookie) error {
	if len(cookies) == 0 {
		return nil
	}

	var options []huh.Option[int]
	var selected []int

	for i, c := range cookies {
		options = append(options, huh.NewOption(fmt.Sprintf("%-20s %s", c.Name, curlparse.MaskValue(c.Value)), i))
		if c.Selected {
			selected = append(selected, i)
		}
	}

	err := huh.NewMultiSelect[int]().
		Title(fmt.Sprintf("Select cookies to save (%d found)", len(cookies))).
		Options(options...).
		Value(&selected).
		Height(15).
		Run()
	if err != nil {
		return err
	}

	applySelection(len(cookies), selected, func(int) bool { return false }, func(i int, v bool) { cookies[i].Selected = v })
	return nil
}

func (p *Prompter) pickCookiesText(cookies []curlparse.Cookie) error {
	if len(cookies) == 0 {
		return nil
	}

	fmt.Fprintf(p.Out, "\nCookies found (%d). Select cookies to save:\n", len(cookies))
	for i, c := range cookies {
		marker := " "
		if c.Selected {
			marker = "x"
		}
		fmt.Fprintf(p.Out, "  %d. [%s] %-20s %s\n", i+1, marker, c.Name, curlparse.MaskValue(c.Value))
	}

	fmt.Fprintf(p.Out, "\nToggle by number (e.g. \"3 5\"), [a]ll, [n]one, or enter to confirm: ")

	answer, err := p.readLine()
	if err != nil {
		return err
	}
	answer = strings.TrimSpace(answer)

	switch strings.ToLower(answer) {
	case "a":
		for i := range cookies {
			cookies[i].Selected = true
		}
	case "n":
		for i := range cookies {
			cookies[i].Selected = false
		}
	case "":
		// keep
	default:
		for _, tok := range strings.Fields(answer) {
			var idx int
			if _, err := fmt.Sscanf(tok, "%d", &idx); err == nil && idx >= 1 && idx <= len(cookies) {
				cookies[idx-1].Selected = !cookies[idx-1].Selected
			}
		}
	}

	selected := 0
	for _, c := range cookies {
		if c.Selected {
			selected++
		}
	}
	fmt.Fprintf(p.Out, "Selected %d of %d cookies\n", selected, len(cookies))
	return nil
}

// applySelection clears all non-skipped items and sets only the selected indices.
func applySelection(total int, selected []int, skip func(int) bool, set func(int, bool)) {
	selectedSet := make(map[int]struct{}, len(selected))
	for _, idx := range selected {
		selectedSet[idx] = struct{}{}
	}
	for i := range total {
		if skip(i) {
			continue
		}
		_, ok := selectedSet[i]
		set(i, ok)
	}
}
